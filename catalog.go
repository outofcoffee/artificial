package main

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// providersYAML is the externalised provider/model-family catalogue, embedded
// into the binary at build time but maintained as a plain file.
//
//go:embed providers.yaml
var providersYAML []byte

// Catalog is the parsed providers.yaml.
type Catalog struct {
	Providers map[string]*Provider `yaml:"providers"`
}

// Provider describes how to construct an opencode provider block.
type Provider struct {
	Description    string             `yaml:"description"`
	Name           string             `yaml:"name"`
	NPM            string             `yaml:"npm"`
	APIKeyEnv      string             `yaml:"apiKeyEnv"`
	APIKeyRequired bool               `yaml:"apiKeyRequired"`
	APIKeyPrefix   string             `yaml:"apiKeyPrefix"`
	Options        map[string]any     `yaml:"options"`
	OptionsFromEnv map[string]string  `yaml:"optionsFromEnv"`
	Families       map[string]*Family `yaml:"families"`
}

// Family is a named set of models within a provider.
type Family struct {
	Description  string                    `yaml:"description"`
	DefaultModel string                    `yaml:"defaultModel"`
	Models       map[string]map[string]any `yaml:"models"`
}

// loadCatalog parses the embedded catalogue.
func loadCatalog() (*Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(providersYAML, &c); err != nil {
		return nil, fmt.Errorf("parsing provider catalogue: %w", err)
	}
	return &c, nil
}

// sortedProviderNames returns provider keys in stable order.
func (c *Catalog) sortedProviderNames() []string {
	names := make([]string, 0, len(c.Providers))
	for n := range c.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// sortedFamilyNames returns a provider's family names in stable order.
func (p *Provider) sortedFamilyNames() []string {
	names := make([]string, 0, len(p.Families))
	for n := range p.Families {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// modelKeys returns a family's model keys in stable order.
func (f *Family) modelKeys() []string {
	keys := make([]string, 0, len(f.Models))
	for k := range f.Models {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// buildProviderBlock turns a provider plus an optional family and/or explicit
// model into an opencode provider block, returning the block and the
// fully-qualified default model (provider/model), or "" if none was selected.
// resolve looks up env vars (typically from .env, then the environment).
func buildProviderBlock(id string, p *Provider, familyName, modelOverride string, resolve func(string) string) (block map[string]any, defaultModel string, err error) {
	block = map[string]any{}
	if p.Name != "" {
		block["name"] = p.Name
	}
	if p.NPM != "" {
		block["npm"] = p.NPM
	}

	options := map[string]any{}
	for k, v := range p.Options {
		options[k] = v
	}
	for optKey, envVar := range p.OptionsFromEnv {
		if v := resolve(envVar); v != "" {
			options[optKey] = v
		}
	}
	if p.APIKeyEnv != "" {
		key := resolve(p.APIKeyEnv)
		switch {
		case key == "" && p.APIKeyRequired:
			return nil, "", fmt.Errorf("%s is not set; add it to your .env or environment", p.APIKeyEnv)
		case key != "" && p.APIKeyPrefix != "" && !strings.HasPrefix(key, p.APIKeyPrefix):
			return nil, "", fmt.Errorf("%s does not look right (expected it to start with %q)", p.APIKeyEnv, p.APIKeyPrefix)
		case key != "":
			options["apiKey"] = key
		}
	}
	if len(options) > 0 {
		block["options"] = options
	}

	models := map[string]any{}
	var defaultModelKey string
	if familyName != "" {
		fam, ok := p.Families[familyName]
		if !ok {
			return nil, "", fmt.Errorf("provider %q has no model family %q (see `oc-config list`)", id, familyName)
		}
		for k, v := range fam.Models {
			models[k] = v
		}
		defaultModelKey = fam.DefaultModel
	}
	if modelOverride != "" {
		if _, ok := models[modelOverride]; !ok {
			models[modelOverride] = map[string]any{"name": modelOverride}
		}
		defaultModelKey = modelOverride
	}
	if len(models) > 0 {
		block["models"] = models
	}

	if defaultModelKey != "" {
		defaultModel = id + "/" + defaultModelKey
	}
	return block, defaultModel, nil
}
