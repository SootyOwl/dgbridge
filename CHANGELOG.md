# Changelog

## 1.0.5

* Strip ANSI color codes from relayed messages.

## 1.0.4

* Added `^N` template parameter for user nicknames.

## 1.0.3

* Added role color handling with fallback to user accent color.

## 1.0.2

* Newlines in Discord messages are now removed
* Minecraft Forge rules removed, as the default Minecraft rules should be compatible with it

### Internal Changes

* Documentation made less verbose
* Versioning is now done through linker flags in the build script (scripts/build.sh)
* Removed Plan9 from the list of Gox's build targets
* Packaging script (package.py) created

## 1.0.1

* Update Discordgo from 0.27.0 to 0.27.1
* Added rules for Minecraft Forge
* Removed "Received signal" console debug line
* Added `^C` template parameter. This new template parameter is replaced by the
  message author's Discord role's hex color code.

## 1.0.0

* Initial release