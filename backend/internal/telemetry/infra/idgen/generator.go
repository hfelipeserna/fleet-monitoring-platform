package idgen

import sharedidgen "fleetmonitoring/backend/internal/shared/idgen"

type Generator struct{}

func New() *Generator { return &Generator{} }

func (g *Generator) NewID() string { return GenerateUUID() }

func GenerateUUID() string { return sharedidgen.GenerateUUID() }
