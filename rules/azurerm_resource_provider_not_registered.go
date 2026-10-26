package rules

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/hclext"
	"github.com/terraform-linters/tflint-plugin-sdk/logger"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
	"github.com/terraform-linters/tflint-ruleset-azurerm/project"
	"github.com/terraform-linters/tflint-ruleset-azurerm/rules/resourceproviders"
	"github.com/zclconf/go-cty/cty"
)

const (
	resourceProviderRegistrationsAttributeName = "resource_provider_registrations"
	resourceProvidersToRegisterAttributeName   = "resource_providers_to_register"
	defaultResourceProviderRegistrations       = "none"
)

// AzurermResourceProviderNotRegisteredRule checks whether the resource providers used by resources are registered
type AzurermResourceProviderNotRegisteredRule struct {
	tflint.DefaultRule
}

type azurermResourceProviderNotRegisteredRuleConfig struct {
	DefaultRegistrations string `hclext:"default_registrations,optional"`
}

// providerRegistrations describes the namespaces an azurerm provider configuration registers.
type providerRegistrations struct {
	name string
	mode string
	// namespaces is nil when the registered namespaces cannot be determined statically.
	namespaces resourceproviders.Set
}

// NewAzurermResourceProviderNotRegisteredRule returns a new rule
func NewAzurermResourceProviderNotRegisteredRule() *AzurermResourceProviderNotRegisteredRule {
	return &AzurermResourceProviderNotRegisteredRule{}
}

// Name returns the rule name
func (r *AzurermResourceProviderNotRegisteredRule) Name() string {
	return "azurerm_resource_provider_not_registered"
}

// Enabled returns whether the rule is enabled by default
func (r *AzurermResourceProviderNotRegisteredRule) Enabled() bool {
	return false
}

// Severity returns the rule severity
func (r *AzurermResourceProviderNotRegisteredRule) Severity() tflint.Severity {
	return tflint.WARNING
}

// Link returns the rule reference link
func (r *AzurermResourceProviderNotRegisteredRule) Link() string {
	return project.ReferenceLink(r.Name())
}

// Check checks whether the resource providers used by azurerm resources are registered by their provider configuration
func (r *AzurermResourceProviderNotRegisteredRule) Check(runner tflint.Runner) error {
	modulePath, err := runner.GetModulePath()
	if err != nil {
		return err
	}
	// Provider configurations live in the root module, so child modules are not checked.
	if !modulePath.IsRoot() {
		return nil
	}

	config := azurermResourceProviderNotRegisteredRuleConfig{}
	if err := runner.DecodeRuleConfig(r.Name(), &config); err != nil {
		return err
	}
	if config.DefaultRegistrations == "" {
		config.DefaultRegistrations = defaultResourceProviderRegistrations
	}
	if _, ok := resourceproviders.Registrations[config.DefaultRegistrations]; !ok {
		return fmt.Errorf(
			"invalid default_registrations %q for rule %s, must be one of: %s",
			config.DefaultRegistrations, r.Name(), strings.Join(slices.Sorted(maps.Keys(resourceproviders.Registrations)), ", "),
		)
	}

	providers, err := loadProviderRegistrations(runner, config.DefaultRegistrations)
	if err != nil {
		return err
	}
	// Without an azurerm provider block, this is most likely a reusable module.
	if providers == nil {
		return nil
	}

	// Resources with count = 0 or an empty for_each are still checked, as they are expected to be created eventually.
	resources, err := runner.GetModuleContent(&hclext.BodySchema{
		Blocks: []hclext.BlockSchema{
			{
				Type:       "resource",
				LabelNames: []string{"type", "name"},
				Body: &hclext.BodySchema{
					Attributes: []hclext.AttributeSchema{{Name: "provider"}},
				},
			},
		},
	}, &tflint.GetModuleContentOption{ExpandMode: tflint.ExpandModeNone})
	if err != nil {
		return err
	}

	for _, resource := range resources.Blocks {
		resourceType := resource.Labels[0]
		required, ok := resourceproviders.Resources[resourceType]
		if !ok {
			continue
		}

		alias, ok := providerAlias(resource)
		if !ok {
			continue
		}
		provider, ok := providers[alias]
		if !ok {
			logger.Debug("skipping resource, provider configuration not found", "resource", resourceType, "alias", alias)
			continue
		}
		if provider.namespaces == nil {
			logger.Debug("skipping resource, registered namespaces are unknown", "resource", resourceType, "provider", provider.name)
			continue
		}

		var missing []string
		for _, namespace := range required {
			if !provider.namespaces.Contains(namespace) {
				missing = append(missing, namespace)
			}
		}
		if len(missing) == 0 {
			continue
		}

		if err := runner.EmitIssue(r, notRegisteredMessage(resourceType, provider, missing), resource.DefRange); err != nil {
			return err
		}
	}

	return nil
}

// loadProviderRegistrations returns the registrations per provider alias, with "" for the default configuration.
// It returns nil when the module has no azurerm provider configuration.
func loadProviderRegistrations(runner tflint.Runner, defaultMode string) (map[string]*providerRegistrations, error) {
	content, err := runner.GetProviderContent("azurerm", &hclext.BodySchema{
		Attributes: []hclext.AttributeSchema{
			{Name: "alias"},
			{Name: resourceProviderRegistrationsAttributeName},
			{Name: resourceProvidersToRegisterAttributeName},
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	if len(content.Blocks) == 0 {
		return nil, nil
	}

	// Resources without a provider argument use the default configuration, which is implicit when only aliased
	// configurations are declared.
	providers := map[string]*providerRegistrations{
		"": {name: "azurerm", mode: defaultMode, namespaces: registeredNamespaces(defaultMode, nil)},
	}

	for _, block := range content.Blocks {
		provider, alias, err := loadProviderRegistration(runner, block, defaultMode)
		if err != nil {
			return nil, err
		}
		providers[alias] = provider
	}

	return providers, nil
}

// loadProviderRegistration returns the registrations of a provider block and its alias.
func loadProviderRegistration(runner tflint.Runner, block *hclext.Block, defaultMode string) (*providerRegistrations, string, error) {
	alias := ""
	if attr, ok := block.Body.Attributes["alias"]; ok {
		if err := runner.EvaluateExpr(attr.Expr, &alias, nil); err != nil {
			return nil, "", err
		}
	}

	provider := &providerRegistrations{name: "azurerm", mode: defaultMode}
	if alias != "" {
		provider.name = "azurerm." + alias
	}

	if attr, ok := block.Body.Attributes[resourceProviderRegistrationsAttributeName]; ok {
		value, err := evaluate(runner, attr.Expr, cty.String)
		if err != nil {
			return nil, "", err
		}
		if !value.IsWhollyKnown() {
			return provider, alias, nil
		}
		if !value.IsNull() {
			provider.mode = value.AsString()
		}
	}

	var extra []string
	if attr, ok := block.Body.Attributes[resourceProvidersToRegisterAttributeName]; ok {
		value, err := evaluate(runner, attr.Expr, cty.List(cty.String))
		if err != nil {
			return nil, "", err
		}
		if !value.IsWhollyKnown() {
			return provider, alias, nil
		}
		if !value.IsNull() {
			for it := value.ElementIterator(); it.Next(); {
				_, namespace := it.Element()
				extra = append(extra, namespace.AsString())
			}
		}
	}

	provider.namespaces = registeredNamespaces(provider.mode, extra)
	return provider, alias, nil
}

// registeredNamespaces returns the namespaces registered in a subscription for the given mode and additional
// namespaces, including those Azure registers by default. It returns nil for an unknown mode, which the azurerm
// provider reports itself.
func registeredNamespaces(mode string, extra []string) resourceproviders.Set {
	namespaces, ok := resourceproviders.RegistrationSet(mode)
	if !ok {
		return nil
	}
	namespaces.Add(resourceproviders.RegisteredByDefault...)
	namespaces.Add(extra...)
	return namespaces
}

// evaluate evaluates an expression to a value of the given type, which may be null or unknown, e.g. when it refers
// to a module output or a variable without a value.
func evaluate(runner tflint.Runner, expr hcl.Expression, wantType cty.Type) (cty.Value, error) {
	var value cty.Value
	err := runner.EvaluateExpr(expr, &value, &tflint.EvaluateExprOption{WantType: &wantType})
	return value, err
}

// providerAlias returns the azurerm provider alias used by a resource, with "" for the default configuration.
// It returns false when the resource uses a provider configuration with another local name.
func providerAlias(resource *hclext.Block) (string, bool) {
	attr, ok := resource.Body.Attributes["provider"]
	if !ok {
		return "", true
	}

	traversal, diags := hcl.AbsTraversalForExpr(attr.Expr)
	if diags.HasErrors() || traversal.RootName() != "azurerm" {
		return "", false
	}
	if len(traversal) < 2 {
		return "", true
	}
	if step, ok := traversal[1].(hcl.TraverseAttr); ok {
		return step.Name, true
	}

	return "", false
}

func notRegisteredMessage(resourceType string, provider *providerRegistrations, missing []string) string {
	quoted := make([]string, len(missing))
	for i, namespace := range missing {
		quoted[i] = fmt.Sprintf("%q", namespace)
	}
	slices.Sort(quoted)

	subject := fmt.Sprintf("The %s resource provider required by %q is", quoted[0], resourceType)
	pronoun := "it"
	if len(quoted) > 1 {
		subject = fmt.Sprintf("The %s resource providers required by %q are", strings.Join(quoted, ", "), resourceType)
		pronoun = "them"
	}

	return fmt.Sprintf(
		"%s not registered by provider %q (resource_provider_registrations = %q). Add %s to resource_providers_to_register.",
		subject, provider.name, provider.mode, pronoun,
	)
}
