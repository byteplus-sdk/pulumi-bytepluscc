package volcenginecc

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	pf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/byteplus-sdk/pulumi-bytepluscc/provider/pkg/version"
)

func TestProviderCredentialsEncrypted(t *testing.T) {
	ctx := context.Background()
	oldVersion := version.Version
	version.Version = "0.0.0"
	t.Cleanup(func() { version.Version = oldVersion })
	info := Provider()
	for _, field := range info.Config {
		if field.Default != nil {
			for _, env := range field.Default.EnvVars {
				t.Setenv(env, "")
			}
		}
	}
	packageSchema, err := os.ReadFile("cmd/pulumi-resource-bytepluscc/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := pf.NewProvider(ctx, info, pf.ProviderMetadata{
		PackageSchema: packageSchema, BridgeMetadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	credentials := map[resource.PropertyKey]string{
		"accessKey":    "BYTEPLUS_ACCESS_KEY",
		"secretKey":    "BYTEPLUS_SECRET_KEY",
		"sessionToken": "BYTEPLUS_SESSION_TOKEN",
	}
	for _, source := range []string{"explicit", "environment", "legacy-state"} {
		t.Run(source, func(t *testing.T) {
			news := resource.PropertyMap{"region": resource.NewStringProperty("ap-southeast-1")}
			olds := resource.PropertyMap{}
			for key, env := range credentials {
				value := resource.NewStringProperty("fake-regression-" + string(key))
				switch source {
				case "environment":
					t.Setenv(env, value.StringValue())
				case "legacy-state":
					olds[key], news[key] = value, value
				default:
					news[key] = value
				}
			}
			checked, err := provider.CheckConfig(ctx, plugin.CheckConfigRequest{
				URN:  "urn:pulumi:test::credentials::pulumi:providers:bytepluscc::test",
				News: news, Olds: olds, AllowUnknowns: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(checked.Failures) != 0 {
				t.Fatalf("CheckConfig returned %d validation failures", len(checked.Failures))
			}
			if checked.Properties["region"].IsSecret() {
				t.Error("region should remain an ordinary configuration value")
			}
			crypter := config.NewSymmetricCrypter(make([]byte, 32))
			for key := range credentials {
				// Use the actual checkpoint serializer: checking only schema flags
				// would miss a lost secret marker on environment defaults.
				serialized, err := stack.SerializePropertyValue(ctx, checked.Properties[key], crypter, false)
				if err != nil {
					t.Fatal(err)
				}
				secret, ok := serialized.(*apitype.SecretV1)
				if !ok {
					t.Errorf("%s would be stored as plaintext in the checkpoint", key)
					continue
				}
				if secret.Ciphertext == "" || secret.Plaintext != "" {
					t.Errorf("%s is missing checkpoint ciphertext", key)
					continue
				}
				plaintext, err := crypter.DecryptValue(ctx, secret.Ciphertext)
				if err != nil {
					t.Fatal(err)
				}
				expected, _ := json.Marshal("fake-regression-" + string(key))
				if plaintext != string(expected) {
					t.Errorf("%s changed during encryption", key)
				}
			}
		})
	}
}

func TestCredentialSchemaSecrets(t *testing.T) {
	data, err := os.ReadFile("cmd/pulumi-resource-bytepluscc/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec schema.PackageSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	for name, properties := range map[string]map[string]schema.PropertySpec{
		"config":           spec.Config.Variables,
		"provider inputs":  spec.Provider.InputProperties,
		"provider outputs": spec.Provider.Properties,
	} {
		for _, key := range []string{"accessKey", "secretKey", "sessionToken"} {
			if !properties[key].Secret {
				t.Errorf("%s.%s must be secret in the generated schema", name, key)
			}
		}
	}
}
