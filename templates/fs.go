// Package templates holds the embedded project skeletons Charlemagne renders.
package templates

import "embed"

// HealthFS is the health-only resttest-shaped service tree.
//
//go:embed all:health
var HealthFS embed.FS
