package main

import (
	"dgbridge/src/lib"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

type CliArgs struct {
	RulesFile string `arg:"required,-r,--rules" help:"Rules to be tested"`
	TestFile  string `arg:"required,-t,--test"  help:"Path to test file"`
	UsersFile string `arg:"-u,--users" help:"Path to the file mapping in-game names to Discord User IDs for mentioning"`
}

func main() {
	fmt.Printf("Dgbridge Rule Tester (v%v)\n", lib.Version)

	//
	// Init global state
	//
	validate = validator.New()

	//
	// Parse CLI args
	//
	var args CliArgs
	arg.MustParse(&args)

	//
	// Load files from CLI parameters
	//
	rules, err := loadRulesFile(args)
	if err != nil {
		printError("Failed to load rules file: %v", err)
		os.Exit(1)
	}
	root, err := loadFileRoot(args)
	if err != nil {
		printError("Failed to load test file: %v", err)
		os.Exit(1)
	}
	userMap, err := loadUsersFile(args)
	if err != nil {
		printError("Failed to load user map file: %v", err)
		os.Exit(1)
	}

	testRunner := NewTestRunner(root, rules, userMap)
	testRunner.RunTests()
}

func loadFileRoot(args CliArgs) (*FileRoot, error) {
	fileContents, err := os.ReadFile(args.TestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load test file: %v", err)
	}
	var test FileRoot
	if err := json.Unmarshal(fileContents, &test); err != nil {
		return nil, fmt.Errorf("error loading test file: %v", err)
	}
	if err := validate.Struct(test); err != nil {
		return nil, fmt.Errorf(
			"validation of test file failed.\n"+
				"Please look at the errors below and try to fix them.\n"+
				"%v", err)
	}
	return &test, nil
}

func loadRulesFile(args CliArgs) (*lib.Rules, error) {
	rules, err := lib.LoadRules(args.RulesFile)
	if err != nil {
		return nil, fmt.Errorf("error loading rules: %v", err)
	}
	if err := validate.Struct(rules); err != nil {
		return nil, fmt.Errorf(
			"validation of rules file failed.\n"+
				"Please look at the errors below and try to fix them.\n"+
				"%v", err)
	}
	return rules, nil
}

func loadUsersFile(args CliArgs) (*lib.UserMap, error) {
	userMap, err := lib.LoadUserMap(args.UsersFile)
	if err != nil {
		return nil, fmt.Errorf("error loading user map: %v", err)
	}
	// Only validate if a user map file was specified.
	// Assumes lib.LoadUserMap returns a valid empty map and nil error if args.UsersFile is empty.
	if args.UsersFile != "" {
		// the UserMap is map[string]string
		// so we need to validate the keys and values
		for key, value := range *userMap {
			if err := validate.Var(key, "required"); err != nil {
				return nil, fmt.Errorf("user map key '%s' is invalid: %v", key, err)
			}
			if err := validate.Var(value, "required"); err != nil {
				return nil, fmt.Errorf("user map value '%s' is invalid: %v", value, err)
			}
		}
		// Validate the map itself
		if err := validate.Var(userMap, "required"); err != nil {
			return nil, fmt.Errorf("user map is invalid: %v", err)
		}
		// Validate the length of the map
		if err := validate.Var(len(*userMap), "gt=0"); err != nil {
			return nil, fmt.Errorf("user map is empty: %v", err)
		}
	}
	return userMap, nil
}

func printError(format string, vargs ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, vargs...)
}
