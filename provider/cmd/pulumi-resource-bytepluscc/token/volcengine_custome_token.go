package token

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	sdkToken "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func VolcengineToken(
	tfPackagePrefix string, finalize tokens.Make,
) tokens.Strategy {
	return tokens.Strategy{
		Resource:   volcengineModule(tfPackagePrefix, knownResource(finalize), camelCase),
		DataSource: volcengineModule(tfPackagePrefix, knownDataSource(finalize), camelCase),
	}
}

func volcengineModule[T info.Resource | info.DataSource](
	prefix string, apply func(string, string, *T, error) error,
	moduleTransform func(string) string,
) info.ElementStrategy[T] {
	return func(tfToken string, elem *T) error {
		var tk string
		if t, foundPrefix := strings.CutPrefix(tfToken, prefix); foundPrefix {
			if t == "" {
				return fmt.Errorf("terraform format error,%s", tfToken)
			} else {
				tk = t
			}
		} else {
			return fmt.Errorf("terraform format error,%s", tfToken)
		}
		parts := strings.SplitN(tk, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("terraform format error,%s", tfToken)
		}
		mod := parts[0]

		return apply(moduleTransform(mod), upperCamelCase(strings.TrimPrefix(tk, mod)), elem, nil)
	}
}

// camelCase converts a TF token a valid Pulumi token segment in camelCase format.
func camelCase(s string) string {
	s = cgstrings.ModifyStringAroundDelimeter(s, "_", cgstrings.UppercaseFirst)

	// Terraform allows both `-` and `_` in it's tokens, but Pulumi allows
	// neither.
	return cgstrings.ModifyStringAroundDelimeter(s, "-", cgstrings.UppercaseFirst)
}
func upperCamelCase(s string) string { return cgstrings.UppercaseFirst(camelCase(s)) }

func knownResource(finalize tokens.Make) func(mod, tk string, r *info.Resource, err error) error {
	return func(mod, tk string, r *info.Resource, err error) error {
		if r.Tok != "" {
			return nil
		}
		if err != nil {
			return err
		}
		tk, err = finalize(mod, tk)
		if err != nil {
			return err
		}

		r.Tok = sdkToken.Type(tk)
		return nil
	}
}

func knownDataSource(finalize tokens.Make) func(mod, tk string, d *info.DataSource, err error) error {
	return func(mod, tk string, d *info.DataSource, err error) error {
		if d.Tok != "" {
			return nil
		}
		if err != nil {
			return err
		}
		tk, err = finalize(mod, "get"+tk)
		if err != nil {
			return err
		}
		d.Tok = sdkToken.ModuleMember(tk)
		return nil
	}
}
