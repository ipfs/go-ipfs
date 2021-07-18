package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/repo"
	. "github.com/ipfs/go-ipfs/repo/common"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/elgris/jsondiff"
	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
)

// ConfigUpdateOutput is config profile apply command's output
type ConfigUpdateOutput struct {
	OldCfg map[string]interface{}
	NewCfg map[string]interface{}
}

type ConfigField struct {
	Key   string
	Value interface{}
}

const (
	configBoolOptionName   = "bool"
	configJSONOptionName   = "json"
	configDryRunOptionName = "dry-run"
)

var ConfigCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and set IPFS config values.",
		ShortDescription: `
'ipfs config' controls configuration variables. It works like 'git config'.
The configuration values are stored in a config file inside your IPFS_PATH.`,
		LongDescription: `
'ipfs config' controls configuration variables. It works
much like 'git config'. The configuration values are stored in a config
file inside your IPFS repository (IPFS_PATH).

Examples:

Get the value of the 'Datastore.Path' key:

  $ ipfs config Datastore.Path

Set the value of the 'Datastore.Path' key:

  $ ipfs config Datastore.Path ~/.ipfs/datastore

Values behind map key names that include dots can be accessed like this:

  $ ipfs config Pinning.RemoteServices["pins.example.org"].Policies

`,
	},
	Subcommands: map[string]*cmds.Command{
		"show":    configShowCmd,
		"edit":    configEditCmd,
		"replace": configReplaceCmd,
		"profile": configProfileCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key of the config entry (e.g. \"Addresses.API\")."),
		cmds.StringArg("value", false, false, "The value to set the config entry to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(configBoolOptionName, "Set a boolean value."),
		cmds.BoolOption(configJSONOptionName, "Parse stringified JSON."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		args := req.Arguments
		key := args[0]

		var output *ConfigField

		// This is a temporary fix until we move the private key out of the config file
		switch strings.ToLower(key) {
		case "identity", "identity.privkey":
			return errors.New("cannot show or change private key through API")
		default:
		}

		// Temporary fix until we move ApiKey secrets out of the config file
		// (remote services are a map, so more advanced blocking is required)
		if blocked := matchesGlobPrefix(key, config.PinningConcealSelector); blocked {
			return errors.New("cannot show or change pinning services credentials")
		}

		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}
		r, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer r.Close()
		if len(args) == 2 {
			value := args[1]

			if parseJSON, _ := req.Options[configJSONOptionName].(bool); parseJSON {
				var jsonVal interface{}
				if err := json.Unmarshal([]byte(value), &jsonVal); err != nil {
					err = fmt.Errorf("failed to unmarshal json. %s", err)
					return err
				}

				output, err = setConfig(r, key, jsonVal)
			} else if isbool, _ := req.Options[configBoolOptionName].(bool); isbool {
				output, err = setConfig(r, key, value == "true")
			} else {
				output, err = setConfig(r, key, value)
			}
		} else {
			output, err = getConfig(r, key)
		}

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ConfigField) error {
			if len(req.Arguments) == 2 {
				return nil
			}

			buf, err := config.HumanOutput(out.Value)
			if err != nil {
				return err
			}
			buf = append(buf, byte('\n'))

			_, err = w.Write(buf)
			return err
		}),
	},
	Type: ConfigField{},
}

// matchesGlobPrefix returns true if and only if the key matches the glob.
// The key is a sequence of string "parts", separated by commas.
// The glob is a sequence of string "patterns".
// matchesGlobPrefix tries to match all of the first K parts to the first K patterns, respectively,
// where K is the length of the shorter of key or glob.
// A pattern matches a part if and only if the pattern is "*" or the lowercase pattern equals the lowercase part.
//
// For example:
//	matchesGlobPrefix("foo.bar", []string{"*", "bar", "baz"}) returns true
//	matchesGlobPrefix("foo.bar.baz", []string{"*", "bar"}) returns true
//	matchesGlobPrefix("foo.bar", []string{"baz", "*"}) returns false
func matchesGlobPrefix(key string, glob []string) bool {
	normalizedKey, _ := ConfigKeyToLookupData(key)
	k := strings.Split(normalizedKey, ".")
	for i, g := range glob {
		if i >= len(k) {
			break
		}
		if g == "*" {
			continue
		}
		if !strings.EqualFold(k[i], g) {
			return false
		}
	}
	return true
}

var configShowCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Output config file contents.",
		ShortDescription: `
'ipfs config show' returns config contents without private keys and secrets.
`,
		LongDescription: `
NOTE: For security reasons, this command will omit your private key and any
access tokens for remote services. If you would like to make a full backup of
your config (private key and secrets included), you must copy the config file
from your IPFS repository (IPFS_PATH).
`,
	},
	Type: make(map[string]interface{}),
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		fname, err := config.Filename(cfgRoot)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadFile(fname)
		if err != nil {
			return err
		}

		var cfg map[string]interface{}
		err = json.Unmarshal(data, &cfg)
		if err != nil {
			return err
		}

		cfg, err = scrubValue(cfg, []string{config.IdentityTag, config.PrivKeyTag})
		if err != nil {
			return err
		}

		cfg, err = scrubOptionalValue(cfg, config.PinningConcealSelector)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &cfg)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *map[string]interface{}) error {
			buf, err := config.HumanOutput(out)
			if err != nil {
				return err
			}
			buf = append(buf, byte('\n'))
			_, err = w.Write(buf)
			return err
		}),
	},
}

// Scrubs value and returns error if missing
func scrubValue(m map[string]interface{}, key []string) (map[string]interface{}, error) {
	return scrubMapInternal(m, key, false)
}

// Scrubs value and returns no error if missing
func scrubOptionalValue(m map[string]interface{}, key []string) (map[string]interface{}, error) {
	return scrubMapInternal(m, key, true)
}

func scrubEither(u interface{}, key []string, okIfMissing bool) (interface{}, error) {
	m, ok := u.(map[string]interface{})
	if ok {
		return scrubMapInternal(m, key, okIfMissing)
	}
	return scrubValueInternal(m, key, okIfMissing)
}

func scrubValueInternal(v interface{}, key []string, okIfMissing bool) (interface{}, error) {
	if v == nil && !okIfMissing {
		return nil, errors.New("failed to find specified key")
	}
	return nil, nil
}

func scrubMapInternal(m map[string]interface{}, key []string, okIfMissing bool) (map[string]interface{}, error) {
	if len(key) == 0 {
		return make(map[string]interface{}), nil // delete value
	}
	n := map[string]interface{}{}
	for k, v := range m {
		if key[0] == "*" || strings.EqualFold(key[0], k) {
			u, err := scrubEither(v, key[1:], okIfMissing)
			if err != nil {
				return nil, err
			}
			if u != nil {
				n[k] = u
			}
		} else {
			n[k] = v
		}
	}
	return n, nil
}

var configEditCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open the config file for editing in $EDITOR.",
		ShortDescription: `
To use 'ipfs config edit', you must have the $EDITOR environment
variable set to your preferred text editor.
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true)),
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		filename, err := config.Filename(cfgRoot)
		if err != nil {
			return err
		}

		return editConfig(filename)
	},
}

var configReplaceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Replace the config with <file>.",
		ShortDescription: `
Make sure to back up the config file first if necessary, as this operation
can't be undone.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "The file to use as the new config."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		r, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer r.Close()

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		return replaceConfig(r, file)
	},
}

var configProfileCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Apply profiles to config.",
		ShortDescription: fmt.Sprintf(`
Available profiles:
%s
`, buildProfileHelp()),
	},

	Subcommands: map[string]*cmds.Command{
		"apply": configProfileApplyCmd,
	},
}

var configProfileApplyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Apply profile to config.",
	},
	Options: []cmds.Option{
		cmds.BoolOption(configDryRunOptionName, "print difference between the current config and the config that would be generated"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("profile", true, false, "The profile to apply to the config."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		profile, ok := config.Profiles[req.Arguments[0]]
		if !ok {
			return fmt.Errorf("%s is not a profile", req.Arguments[0])
		}

		dryRun, _ := req.Options[configDryRunOptionName].(bool)
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		oldCfg, newCfg, err := transformConfig(cfgRoot, req.Arguments[0], profile.Transform, dryRun)
		if err != nil {
			return err
		}

		oldCfgMap, err := scrubPrivKey(oldCfg)
		if err != nil {
			return err
		}

		newCfgMap, err := scrubPrivKey(newCfg)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ConfigUpdateOutput{
			OldCfg: oldCfgMap,
			NewCfg: newCfgMap,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ConfigUpdateOutput) error {
			diff := jsondiff.Compare(out.OldCfg, out.NewCfg)
			buf := jsondiff.Format(diff)

			_, err := w.Write(buf)
			return err
		}),
	},
	Type: ConfigUpdateOutput{},
}

func buildProfileHelp() string {
	var out string

	for name, profile := range config.Profiles {
		dlines := strings.Split(profile.Description, "\n")
		for i := range dlines {
			dlines[i] = "    " + dlines[i]
		}

		out = out + fmt.Sprintf("  '%s':\n%s\n", name, strings.Join(dlines, "\n"))
	}

	return out
}

// scrubPrivKey scrubs private key for security reasons.
func scrubPrivKey(cfg *config.Config) (map[string]interface{}, error) {
	cfgMap, err := config.ToMap(cfg)
	if err != nil {
		return nil, err
	}

	cfgMap, err = scrubValue(cfgMap, []string{config.IdentityTag, config.PrivKeyTag})
	if err != nil {
		return nil, err
	}

	return cfgMap, nil
}

// transformConfig returns old config and new config instead of difference between them,
// because apply command can provide stable API through this way.
// If dryRun is true, repo's config should not be updated and persisted
// to storage. Otherwise, repo's config should be updated and persisted
// to storage.
func transformConfig(configRoot string, configName string, transformer config.Transformer, dryRun bool) (*config.Config, *config.Config, error) {
	r, err := fsrepo.Open(configRoot)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	oldCfg, err := r.Config()
	if err != nil {
		return nil, nil, err
	}

	// make a copy to avoid updating repo's config unintentionally
	newCfg, err := oldCfg.Clone()
	if err != nil {
		return nil, nil, err
	}

	err = transformer(newCfg)
	if err != nil {
		return nil, nil, err
	}

	if !dryRun {
		_, err = r.BackupConfig("pre-" + configName + "-")
		if err != nil {
			return nil, nil, err
		}

		err = r.SetConfig(newCfg)
		if err != nil {
			return nil, nil, err
		}
	}

	return oldCfg, newCfg, nil
}

func getConfig(r repo.Repo, key string) (*ConfigField, error) {
	value, err := r.GetConfigKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get config value: %q", err)
	}
	return &ConfigField{
		Key:   key,
		Value: value,
	}, nil
}

func setConfig(r repo.Repo, key string, value interface{}) (*ConfigField, error) {
	err := r.SetConfigKey(key, value)
	if err != nil {
		return nil, fmt.Errorf("failed to set config value: %s (maybe use --json?)", err)
	}
	return getConfig(r, key)
}

func editConfig(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("ENV variable $EDITOR not set")
	}

	cmd := exec.Command("sh", "-c", editor+" "+filename)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func replaceConfig(r repo.Repo, file io.Reader) error {
	var newCfg config.Config
	if err := json.NewDecoder(file).Decode(&newCfg); err != nil {
		return errors.New("failed to decode file as config")
	}

	// Handle Identity.PrivKey (secret)

	if len(newCfg.Identity.PrivKey) != 0 {
		return errors.New("setting private key with API is not supported")
	}

	keyF, err := getConfig(r, config.PrivKeySelector)
	if err != nil {
		return errors.New("failed to get PrivKey")
	}

	pkstr, ok := keyF.Value.(string)
	if !ok {
		return errors.New("private key in config was not a string")
	}

	newCfg.Identity.PrivKey = pkstr

	// Handle Pinning.RemoteServices (API.Key of each service is a secret)

	newServices := newCfg.Pinning.RemoteServices
	oldServices, err := getRemotePinningServices(r)
	if err != nil {
		return fmt.Errorf("failed to load remote pinning services info (%v)", err)
	}

	// fail fast if service lists are obviously different
	if len(newServices) != len(oldServices) {
		return errors.New("cannot add or remove remote pinning services with 'config replace'")
	}

	// re-apply API details and confirm every modified service already existed
	for name, oldSvc := range oldServices {
		if newSvc, hadSvc := newServices[name]; hadSvc {
			// fail if input changes any of API details
			// (interop with config show: allow Endpoint as long it did not change)
			if len(newSvc.API.Key) != 0 || (len(newSvc.API.Endpoint) != 0 && newSvc.API.Endpoint != oldSvc.API.Endpoint) {
				return errors.New("cannot change remote pinning services api info with `config replace`")
			}
			// re-apply API details and store service in updated config
			newSvc.API = oldSvc.API
			newCfg.Pinning.RemoteServices[name] = newSvc
		} else {
			// error on service rm attempt
			return errors.New("cannot add or remove remote pinning services with 'config replace'")
		}
	}

	return r.SetConfig(&newCfg)
}

func getRemotePinningServices(r repo.Repo) (map[string]config.RemotePinningService, error) {
	var oldServices map[string]config.RemotePinningService
	if remoteServicesTag, err := getConfig(r, config.RemoteServicesPath); err == nil {
		// seems that golang cannot type assert map[string]interface{} to map[string]config.RemotePinningService
		// so we have to manually copy the data :-|
		if val, ok := remoteServicesTag.Value.(map[string]interface{}); ok {
			jsonString, err := json.Marshal(val)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(jsonString, &oldServices)
			if err != nil {
				return nil, err
			}
		}
	}
	return oldServices, nil

}
