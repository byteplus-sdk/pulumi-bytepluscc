package shim

import (
	"github.com/byteplus-sdk/terraform-provider-bytepluscc/internal/provider"
	tfpf "github.com/hashicorp/terraform-plugin-framework/provider"
)

func NewProvider() tfpf.Provider {
	return provider.New()
}
