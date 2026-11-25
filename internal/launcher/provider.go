package launcher

import (
	"fmt"
	"strings"
)

// Provider is a model provider from OpenCode's catalog, with the abliterated model the
// launcher runs on it unless told otherwise.
type Provider struct {
	Name   string // the --provider value, and the credential store account
	Label  string // the name people know the provider by
	ID     string // OpenCode's provider id
	KeyVar string // the environment variable OpenCode reads the key from
	Model  string // the default model id within the provider
	SignIn bool   // whether a key can be obtained through the provider's OAuth sign-in
}

var providers = []Provider{
	{
		Name:   "nanogpt",
		Label:  "NanoGPT",
		ID:     "nano-gpt",
		KeyVar: "NANO_GPT_API_KEY",
		Model:  "pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking",
		SignIn: true,
	},
	{
		Name:   "nearai",
		Label:  "NEAR AI",
		ID:     "nearai",
		KeyVar: "NEARAI_API_KEY",
		Model:  "deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated",
	},
}

// DefaultProvider is the provider used when none is selected.
const DefaultProvider = "nanogpt"

// LookupProvider returns the provider named name, or an error naming the known ones.
func LookupProvider(name string) (Provider, error) {
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		if p.Name == name {
			return p, nil
		}
		names = append(names, p.Name)
	}
	return Provider{}, fmt.Errorf("unknown provider %q; known providers: %s", name, strings.Join(names, ", "))
}

// QualifiedModel returns model as OpenCode names it, "<provider id>/<model id>". An empty
// model is the provider's default; a model already prefixed with the provider id is kept.
func (p Provider) QualifiedModel(model string) string {
	if model == "" {
		model = p.Model
	}
	return p.ID + "/" + strings.TrimPrefix(model, p.ID+"/")
}
