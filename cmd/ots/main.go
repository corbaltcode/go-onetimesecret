package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"
	ots "github.com/corbaltcode/go-onetimesecret"
	"golang.org/x/term"
)

const stdinArg = "-"

type usageErr string

func (e usageErr) Error() string {
	return string(e)
}

type config struct {
	Username string
	Key      string
}

type cmd interface {
	AddFlags(*flag.FlagSet)
	Run(cmdContext, []string) error
}

type cmdContext struct {
	JSON   bool
	Client *ots.Client
}

type cmdType struct {
	Name    string
	Params  string
	Summary string
	Help    string
	NewCmd  func() cmd
}

func (c *cmdType) Usage() string {
	return usage(c.Name, c.Params)
}

var relativeConfigPath = filepath.Join("ots", "config.toml")

var cmdTypes = []cmdType{
	{
		Name:    "burn",
		Params:  "[-passphrase <string>] metadata-key",
		Summary: "Destroys a secret",
		Help:    "Destroys a secret. Prints the destroyed secret's metadata key. If passphrase is \"-\", reads a line from stdin.",
		NewCmd: func() cmd {
			return &burnCmd{}
		},
	},
	{
		Name:    "gen",
		Params:  "[-passphrase <string>] [-ttl <seconds>]",
		Summary: "Generates a secret",
		Help:    "Generates a secret. Prints the secret, secret key, and metadata key. If passphrase is \"-\", reads a line from stdin.",
		NewCmd: func() cmd {
			return &generateCmd{}
		},
	},
	{
		Name:    "get",
		Params:  "[-passphrase <string>] secret-key",
		Summary: "Retrieves a secret",
		Help:    "Retrieves, prints, and destroys a secret. If passphrase is \"-\", reads a line from stdin.",
		NewCmd: func() cmd {
			return &getCmd{}
		},
	},
	{
		Name:    "meta",
		Params:  "metadata-key",
		Summary: "Prints a secret's metadata",
		Help:    "Prints a secret's metadata.",
		NewCmd: func() cmd {
			return &metadataCmd{}
		},
	},
	{
		Name:    "put",
		Summary: "Stores a secret",
		Help:    "Stores a secret. Prints the secret key and metadata key. If passphrase is \"-\", reads a line from stdin. If secret is \"-\", reads a line from stdin or, if stdin is not a terminal, reads until EOF.",
		Params:  "[-passphrase <string>] [-ttl <int>] secret",
		NewCmd: func() cmd {
			return &putCmd{}
		},
	},
	{
		Name:    "recent",
		Summary: "Prints metadata of recently created secrets",
		Help:    "Prints metadata of recently created secrets.",
		NewCmd: func() cmd {
			return &recentCmd{}
		},
	},
	{
		Name:    "status",
		Summary: "Prints system status",
		Help:    "Prints system status.",
		NewCmd: func() cmd {
			return &statusCmd{}
		},
	},
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printHelp(os.Stderr)
		os.Exit(1)
	}

	cmdName := os.Args[1]

	if contains([]string{"help", "-h", "-help", "--help"}, cmdName) {
		if len(os.Args) < 3 {
			printHelp(os.Stdout)
			os.Exit(0)
		}

		cmdType, err := findCmdType(os.Args[2])
		if err != nil {
			log.Printf("Unknown command: %v\n", os.Args[2])
			log.Println("Run 'ots help' for usage.")
			os.Exit(1)
		}

		fmt.Println(cmdType.Usage())
		fmt.Println("")
		fmt.Println(cmdType.Help)
		os.Exit(0)
	}

	cmdType, err := findCmdType(cmdName)
	if err != nil {
		log.Printf("Unknown command: %v\n", cmdName)
		log.Println("Run 'ots help' for usage.")
		os.Exit(1)
	}
	cmd := cmdType.NewCmd()

	var client ots.Client
	var ctx cmdContext
	ctx.Client = &client

	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.SetOutput(&bytes.Buffer{}) // tell flags not to print errors; we'll do that
	flags.StringVar(&client.Username, "username", "", "")
	flags.StringVar(&client.Key, "key", "", "")
	flags.BoolVar(&ctx.JSON, "json", false, "")
	cmd.AddFlags(flags)

	err = flags.Parse(os.Args[2:])
	if err != nil {
		log.Println(err)
		log.Println(cmdType.Usage())
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("error reading config: %v\n", err)
	}

	if client.Username == "" {
		client.Username = os.Getenv("OTS_USERNAME")
	}
	if client.Username == "" {
		client.Username = cfg.Username
	}
	if client.Username == "" {
		log.Fatalln("missing username; run 'ots help'")
	}

	if client.Key == "" {
		client.Key = os.Getenv("OTS_KEY")
	}
	if client.Key == "" {
		client.Key = cfg.Key
	}
	if client.Key == "" {
		log.Fatalln("missing key; run 'ots help'")
	}

	err = cmd.Run(ctx, flags.Args())

	if err != nil {
		log.Println(err)
		_, ok := err.(usageErr)
		if ok {
			log.Println(cmdType.Usage())
		}
		os.Exit(1)
	}
}

type burnCmd struct {
	passphrase string
}

func (c *burnCmd) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.passphrase, "passphrase", "", "")
}

func (c *burnCmd) Run(ctx cmdContext, args []string) error {
	if len(args) < 1 {
		return usageErr("missing arg: metadata-key")
	} else if len(args) > 1 {
		return usageErr("too many args")
	}

	if c.passphrase == stdinArg {
		if err := readSecretShort(&c.passphrase, "passphrase"); err != nil {
			return err
		}
	}

	metadataKey := args[0]
	meta, err := ctx.Client.Burn(metadataKey, c.passphrase)
	if err != nil {
		return err
	}

	result := struct {
		MetadataKey string
	}{meta.MetadataKey}

	printResult(result, ctx.JSON)
	return nil
}

type generateCmd struct {
	passphrase string
	secretTTL  int
}

func (c *generateCmd) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.passphrase, "passphrase", "", "")
	flags.IntVar(&c.secretTTL, "ttl", 0, "")
}

func (c *generateCmd) Run(ctx cmdContext, args []string) error {
	if len(args) > 0 {
		return usageErr("too many args")
	}

	if c.passphrase == stdinArg {
		if err := readSecretShort(&c.passphrase, "passphrase"); err != nil {
			return err
		}
	}

	secret, meta, err := ctx.Client.Generate(c.passphrase, c.secretTTL, "")
	if err != nil {
		return err
	}

	result := struct {
		Secret      string
		SecretKey   string
		MetadataKey string
	}{secret, meta.SecretKey, meta.MetadataKey}

	printResult(result, ctx.JSON)
	return nil
}

type getCmd struct {
	passphrase string
}

func (c *getCmd) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.passphrase, "passphrase", "", "")
}

func (c *getCmd) Run(ctx cmdContext, args []string) error {
	if len(args) < 1 {
		return usageErr("missing arg: secret-key")
	} else if len(args) > 1 {
		return usageErr("too many args")
	}

	if c.passphrase == stdinArg {
		if err := readSecretShort(&c.passphrase, "passphrase"); err != nil {
			return err
		}
	}

	secretKey := args[0]
	secret, err := ctx.Client.Get(secretKey, c.passphrase)
	if err != nil {
		return err
	}

	result := struct {
		Secret string
	}{secret}

	printResult(result, ctx.JSON)
	return nil
}

type metadataCmd struct {
}

func (c *metadataCmd) AddFlags(flags *flag.FlagSet) {
}

func (c *metadataCmd) Run(ctx cmdContext, args []string) error {
	if len(args) < 1 {
		return usageErr("missing arg: metadata-key")
	} else if len(args) > 1 {
		return usageErr("too many args")
	}

	metadataKey := args[0]
	meta, err := ctx.Client.GetMetadata(metadataKey)
	if err != nil {
		return err
	}

	printResult(meta, ctx.JSON)
	return nil
}

type putCmd struct {
	passphrase string
	secretTTL  int
}

func (c *putCmd) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.passphrase, "passphrase", "", "")
	flags.IntVar(&c.secretTTL, "ttl", 0, "")
}

func (c *putCmd) Run(ctx cmdContext, args []string) error {
	if len(args) > 1 {
		return usageErr("too many args")
	}

	if c.passphrase == stdinArg {
		if err := readSecretShort(&c.passphrase, "passphrase"); err != nil {
			return err
		}
	}

	var secret string
	if len(args) > 0 {
		secret = args[0]
	} else {
		if err := readSecretLong(&secret, "secret"); err != nil {
			return err
		}
	}

	meta, err := ctx.Client.Put(secret, c.passphrase, c.secretTTL, "")
	if err != nil {
		return err
	}

	result := struct {
		SecretKey   string
		MetadataKey string
	}{meta.SecretKey, meta.MetadataKey}

	printResult(result, ctx.JSON)
	return nil
}

type recentCmd struct {
}

func (c *recentCmd) AddFlags(flags *flag.FlagSet) {
}

func (c *recentCmd) Run(ctx cmdContext, args []string) error {
	if len(args) > 0 {
		return usageErr("too many args")
	}

	metas, err := ctx.Client.GetRecentMetadata()
	if err != nil {
		return err
	}

	printResult(metas, ctx.JSON)
	return nil
}

type statusCmd struct {
}

func (c *statusCmd) AddFlags(flags *flag.FlagSet) {
}

func (c *statusCmd) Run(ctx cmdContext, args []string) error {
	if len(args) > 0 {
		return usageErr("too many args")
	}

	status, err := ctx.Client.GetSystemStatus()
	if err != nil {
		return err
	}

	result := struct {
		Status string
	}{string(status)}

	printResult(result, ctx.JSON)
	return nil
}

func contains(strings []string, s string) bool {
	for _, t := range strings {
		if s == t {
			return true
		}
	}
	return false
}

func findCmdType(name string) (cmdType, error) {
	for _, t := range cmdTypes {
		if t.Name == name {
			return t, nil
		}
	}
	return cmdType{}, fmt.Errorf("unknown command: %v", name)
}

func getConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, relativeConfigPath), nil
}

func loadConfig() (config, error) {
	path, err := getConfigPath()
	if err != nil {
		return config{}, err
	}
	var cfg config
	_, err = toml.DecodeFile(path, &cfg)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config{}, fmt.Errorf("invalid config file '%v': %w", path, err)
	}
	return cfg, nil
}

func readSecretShort(v *string, prompt string) error {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return readSecretFromTerminal(v, prompt)
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
		}
		*v = scanner.Text()
		return nil
	}
}

func readSecretLong(v *string, prompt string) error {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return readSecretFromTerminal(v, prompt)
	} else {
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		*v = string(bytes)
		return nil
	}
}

func readSecretFromTerminal(v *string, prompt string) error {
	fmt.Printf("%v: ", prompt)
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println("")
	if err != nil {
		return err
	}
	*v = string(bytes)
	return nil
}

func printResult(v interface{}, json bool) {
	if json {
		if err := printResultJSON(v); err != nil {
			panic(err)
		}
	} else {
		printResultPlain(v)
	}
}

func printResultPlain(v interface{}) {
	val := reflect.ValueOf(v)

	if val.Type() == reflect.TypeOf(time.Time{}) {
		t := val.Interface().(time.Time)
		fmt.Print(t.Format(time.RFC3339))
	} else if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			printResultPlain(val.Index(i).Interface())
		}
	} else if val.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			if i > 0 {
				fmt.Print("\t")
			}
			printResultPlain(val.Field(i).Interface())
		}
		fmt.Print("\n")
	} else {
		fmt.Print(val)
	}
}

func printResultJSON(v interface{}) error {
	json, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	fmt.Print(string(json))
	return nil
}

func usage(cmd string, cmdArgs string) string {
	s := fmt.Sprintf("Usage: ots %v [-username <string>] [-key <string>] [-json]", cmd)
	if len(cmdArgs) > 0 {
		s += " " + cmdArgs
	}
	return s
}

func printHelp(w io.Writer) {
	configPath, err := getConfigPath()
	if err != nil {
		configPath = filepath.Join("$XDG_CONFIG_HOME", relativeConfigPath)
	}

	tw := tabwriter.NewWriter(w, 0, 4, 4, ' ', 0)

	fmt.Fprintln(w, "ots is a command-line interface to One-Time Secret (onetimesecret.com).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, usage("<command>", "<command args>"))
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "")
	for _, t := range cmdTypes {
		tw.Write([]byte(fmt.Sprintf("  %v\t%v\n", t.Name, t.Summary)))
	}
	tw.Flush()
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run \"ots help <command>\" for help on each command.")
	fmt.Fprintln(w, "")

	fmt.Fprintf(w, "ots requires a username and API key from onetimesecret.com. Provide these with the -username and -key options, in the environment variables OTS_USERNAME and OTS_KEY, or in the config file \"%v\". For example:\n", configPath)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  username = \"my-username\"")
	fmt.Fprintln(w, "  key = \"my-key\"")
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "If -json is specified, ots prints JSON.")
}
