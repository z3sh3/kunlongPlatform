// Package modules aggregates the independent console business modules.
// The module packages must not import each other; shared code lives in
// modules/shared.
package modules

import (
	"github.com/QuantumNous/new-api/modules/agentworkshop"
	"github.com/QuantumNous/new-api/modules/datasetmarket"
	"github.com/QuantumNous/new-api/modules/enterprise"
	"github.com/QuantumNous/new-api/modules/modelsquare"
	"github.com/QuantumNous/new-api/modules/talent"
)

// MigrateAll runs the schema migration of every console module. It is
// called from main after model.InitDB, and must not be imported by any
// individual module package.
func MigrateAll() error {
	for _, migrate := range []func() error{
		datasetmarket.Migrate,
		modelsquare.Migrate,
		agentworkshop.Migrate,
		talent.Migrate,
		enterprise.Migrate,
	} {
		if err := migrate(); err != nil {
			return err
		}
	}
	return nil
}
