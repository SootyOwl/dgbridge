package main

import (
	"dgbridge/src/ext"
	"dgbridge/src/lib"
	"fmt"
	"log"
	"regexp" // Added import
	"sort"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Added regex to strip ANSI color codes
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// BotParameters holds data to be passed to StartDiscordBot.
type BotParameters struct {
	Token          string             // Discord auth token
	RelayChannelId string             // Saved in BotContext
	Subprocess     *SubprocessContext // Saved in BotContext
	Rules          lib.Rules          // Saved in BotContext
}

type BotContext struct {
	relayChannelId string             // ID of destination Discord channel
	subprocess     *SubprocessContext // Subprocess context
	rules          lib.Rules          // Message conversion rules
	readyOnce      sync.Once          // Tracks if bot was initialized
}

// StartDiscordBot starts the discord bot. This function is non-blocking.
//
// Returns:
//
//	a function that when called will close the discord bot session, or an
//	error if an error occurs while starting the bot
func StartDiscordBot(params BotParameters) (func(), error) {
	dg, err := discordgo.New("Bot " + params.Token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %v", err)
	}
	context := BotContext{
		relayChannelId: params.RelayChannelId,
		subprocess:     params.Subprocess,
		rules:          params.Rules,
		readyOnce:      sync.Once{},
	}
	dg.AddHandler(context.ready())
	dg.AddHandler(context.messageCreate())
	dg.Identify.Intents = discordgo.IntentsGuildMessages
	err = dg.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %v", err)
	}
	return func() {
		_ = dg.Close()
	}, nil
}

// Handles a discordgo.Ready event.
// Sets up the jobs to relay text to Discord.
func (self *BotContext) ready() func(s *discordgo.Session, r *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		self.readyOnce.Do(func() {
			go self.startRelayJob(s, &self.subprocess.StdoutLineEvent)
			go self.startRelayJob(s, &self.subprocess.StderrLineEvent)
		})
	}
}

// Relays the output of a subprocess to a discord channel.
// It continuously listens to the specified event for data to relay.
//
// If an error occurs when sending a message to Discord, error is simply
// logged to stdout.
//
// Parameters:
//
//	s:
//		A pointer to a discordgo session, used to send the message to discord
//		channel.
//	event:
//		Which subprocess event to listen to
func (self *BotContext) startRelayJob(session *discordgo.Session, event *ext.EventChannel[string]) {
	lineCh := event.Listen()
	defer event.Off(lineCh)
	for line := range lineCh {
		line = lib.ApplyRules(self.rules.SubprocessToDiscord, nil, line)
		if line == "" {
			// No rules matched.
			continue
		}
		// Strip ANSI color codes from the line before sending it to Discord
		// This is necessary to avoid sending raw ANSI codes to Discord, which are
		// ugly, but still allows the subprocess to use colors and the rules to match
		// using ANSI codes.
		line = ansiRegex.ReplaceAllString(line, "")
		// Send the message to the Discord channel
		_, err := session.ChannelMessageSend(self.relayChannelId, line)
		if err != nil {
			log.Printf("error sending message to discord: %v", err)
		}
	}
}

// getHighestRoleWithColor finds the highest positioned role with a color for the member.
// It returns the color value (int) or 0 if no colored role is found or an error occurs.
func getHighestRoleWithColor(s *discordgo.Session, m *discordgo.MessageCreate) int {
	// Ensure member and guild information is available
	if m.Member == nil || m.GuildID == "" || len(m.Member.Roles) == 0 {
		return 0 // Cannot determine role color without member/guild/roles info
	}

	// Fetch all roles for the guild
	guildRoles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		log.Printf("error fetching guild roles for guild %s: %v", m.GuildID, err)
		return 0 // Error fetching roles, cannot determine color
	}

	// Create a map for quick lookup of role details by ID
	roleMap := make(map[string]*discordgo.Role, len(guildRoles))
	for _, role := range guildRoles {
		roleMap[role.ID] = role
	}

	// Filter member's roles to find those with colors
	coloredRoles := make([]*discordgo.Role, 0)
	for _, roleID := range m.Member.Roles {
		if role, ok := roleMap[roleID]; ok && role.Color != 0 {
			coloredRoles = append(coloredRoles, role)
		}
	}

	// If no colored roles were found for the member
	if len(coloredRoles) == 0 {
		return 0
	}

	// Sort the colored roles by position (highest first)
	sort.Slice(coloredRoles, func(i, j int) bool {
		return coloredRoles[i].Position > coloredRoles[j].Position
	})

	// Return the color of the highest positioned role
	return coloredRoles[0].Color
}

// getAccentColor determines the accent color based on the user's highest role or default accent color.
func getAccentColor(s *discordgo.Session, m *discordgo.MessageCreate) int {
	// Try to get the color from the highest role
	roleColor := getHighestRoleWithColor(s, m)
	if roleColor != 0 {
		return roleColor
	}

	// Fallback to the user's profile accent color if available
	if m.Author.AccentColor != 0 {
		return m.Author.AccentColor
	}

	// Default color if no role color or profile accent color is found
	return 0 // Or some other default color value if desired
}

func (self *BotContext) messageCreate() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			// Is bot's own message
			return
		}
		if !(m.ChannelID == self.relayChannelId) {
			// Is not relay channel
			return
		}
		msg := m.Content
		props := &lib.Props{
			Author: lib.Author{
				Username:      m.Author.Username,
				Nickname:      m.Member.Nick,
				Discriminator: m.Author.Discriminator,
				AccentColor:   getAccentColor(s, m),
			},
		}

		// Apply conversion rules
		msg = lib.ApplyRules(self.rules.DiscordToSubprocess, props, msg)
		if msg == "" {
			// No rules matched or message was filtered out.
			return
		}

		// Relay the processed message to the subprocess stdin
		self.subprocess.WriteStdinLineEvent.Broadcast(msg + "\n")
	}
}
