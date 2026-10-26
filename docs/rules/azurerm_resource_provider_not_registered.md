# azurerm_resource_provider_not_registered

Require that the Azure resource providers used by azurerm resources are registered by their provider configuration.

## Configuration

```hcl
rule "azurerm_resource_provider_not_registered" {
  enabled = true

  # (Optional) Mode to assume when resource_provider_registrations is not set. Defaults to "none", the azurerm 5.x default.
  # Set this when you configure the mode with the ARM_RESOURCE_PROVIDER_REGISTRATIONS environment variable instead.
  default_registrations = "none"
}
```

## Example

```hcl
provider "azurerm" {
  resource_provider_registrations = "core"
  resource_providers_to_register  = ["Microsoft.Dashboardd"]

  features {}
}

resource "azurerm_dashboard_grafana" "this" {
  grafana_major_version = 11
  location              = "westeurope"
  name                  = "dg-example"
  resource_group_name   = "rg-example"
}
```

```
$ tflint
1 issue(s) found:

Warning: The "Microsoft.Dashboard" resource provider required by "azurerm_dashboard_grafana" is not registered by provider "azurerm" (resource_provider_registrations = "core"). Add it to resource_providers_to_register. (azurerm_resource_provider_not_registered)

  on main.tf line 8:
   8: resource "azurerm_dashboard_grafana" "this" {

Reference: https://github.com/terraform-linters/tflint-ruleset-azurerm/blob/v0.32.0/docs/rules/azurerm_resource_provider_not_registered.md
```

## Why

Since azurerm 5.0, `resource_provider_registrations` defaults to `none`, and the built-in sets (`core`, `extended`, `all`, `legacy`) are no longer updated. Resources whose resource provider is not registered in the subscription pass `terraform plan`, but fail during `terraform apply` with `MissingSubscriptionRegistration`. A mistyped namespace in `resource_providers_to_register` is ignored by the provider with only a warning in the debug log.

The rule looks up the namespaces each resource uses, based on the `API Providers` section of the azurerm resource documentation. It reports namespaces that are not registered by the `resource_provider_registrations` mode or `resource_providers_to_register` of the provider configuration the resource uses. Namespaces that Azure registers in every subscription, such as `Microsoft.Resources` and `Microsoft.Authorization`, are ignored. Namespaces are compared case-insensitively.

List every namespace the configuration uses, even if another configuration targeting the same subscription already registers it. The azurerm provider skips namespaces that are already registered and never unregisters them, so overlapping lists are harmless.

The rule does not report anything when:

- the module has no `azurerm` provider configuration, such as a reusable module;
- `resource_provider_registrations` or `resource_providers_to_register` cannot be evaluated, for example when it refers to a module output;
- the resource type has no `API Providers` section in the azurerm documentation;
- the resource is in a child module.

## How To Fix

Add the reported namespaces to `resource_providers_to_register` of the provider configuration the resource uses.
