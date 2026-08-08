package check

import (
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/model"
)

// The design read-surface CONTRACT lives in package model (WS1-043) so consumers depend on the
// interface, not this implementation. These aliases keep the historical check.* names — every rule,
// test, and external caller that referenced check.Model / check.ComponentClass / ... is unchanged,
// because a type alias is the same type. check.NewModel* still return model.Model, and irModel
// implements it (asserted below).
type (
	Model          = model.Model
	ComponentClass = model.ComponentClass
	PinRole        = model.PinRole
	PinInst        = model.PinInst
	PinNetConflict = model.PinNetConflict
	BoardNet       = model.BoardNet
	BoardSeg       = model.BoardSeg
	BoardVia       = model.BoardVia
	Reach          = model.Reach
	ReachStep      = model.ReachStep

	// The classification lexicon (WS3-070) lives in package classify (WS3-071) so the loader can run
	// the ingestion pass without importing check. These aliases keep the historical check.* names for
	// the --conventions loader (check/naming) and external overlays.
	ClassVocab    = classify.ClassVocab
	VocabPatterns = classify.VocabPatterns
	// RoleVocabConfig is the named-field override set for BuildRoleVocab (WS3-117).
	RoleVocabConfig = classify.RoleVocabConfig

	// The naming (role) lexicon moved to classify in WS3-072 for the same reason (the net.role stamp
	// runs at ingestion). RoleVocab keeps its check.* name here.
	RoleVocab = classify.RoleVocab

	// Lexicon pairs the two vocabularies as the value a design is READ with (WS3-106), so a caller
	// composing project conventions holds one thing rather than two.
	Lexicon = classify.Lexicon
)

// The classification-lexicon functions, re-exported from classify. SetActiveClassVocab/ActiveClassVocab
// address the same process-global vocab the ingestion pass reads, so a --conventions class override set
// through check.SetActiveClassVocab takes effect at ReadDesign time.
var (
	DefaultClassVocab   = classify.DefaultClassVocab
	BuildClassVocab     = classify.BuildClassVocab
	SetActiveClassVocab = classify.SetActiveClassVocab
	ActiveClassVocab    = classify.ActiveClassVocab
	ParseComponentClass = classify.ParseComponentClass

	// The role-lexicon functions, re-exported from classify (WS3-072). SetActiveRoleVocab/ActiveRoleVocab
	// address the same process-global the net.role stamp reads, so a --conventions role override set
	// through check.SetActiveRoleVocab takes effect at ReadDesign time (StampNetRoles).
	DefaultRoleVocab   = classify.DefaultRoleVocab
	BuildRoleVocab     = classify.BuildRoleVocab
	SetActiveRoleVocab = classify.SetActiveRoleVocab
	ActiveRoleVocab    = classify.ActiveRoleVocab
)

// The net-role tokens ir.Net.roles carries, re-exported from classify so check code reads a role by a
// check.NetRole* name (the fallback helpers and the fact projectors).
const (
	NetRoleRail     = classify.NetRoleRail
	NetRoleGround   = classify.NetRoleGround
	NetRoleFeedback = classify.NetRoleFeedback
)

// The component.class and pin-role vocabularies, re-exported from model.
const (
	ClassResistor             = model.ClassResistor
	ClassCapacitor            = model.ClassCapacitor
	ClassInductor             = model.ClassInductor
	ClassFerrite              = model.ClassFerrite
	ClassDiode                = model.ClassDiode
	ClassLED                  = model.ClassLED
	ClassTVS                  = model.ClassTVS
	ClassZener                = model.ClassZener
	ClassFuse                 = model.ClassFuse
	ClassConnector            = model.ClassConnector
	ClassTestConnector        = model.ClassTestConnector
	ClassTestPoint            = model.ClassTestPoint
	ClassClock                = model.ClassClock
	ClassOscillator           = model.ClassOscillator
	ClassCrystal              = model.ClassCrystal
	ClassCeramicResonator     = model.ClassCeramicResonator
	ClassIC                   = model.ClassIC
	ClassTransistor           = model.ClassTransistor
	ClassIdealDiodeController = model.ClassIdealDiodeController
	ClassUnknown              = model.ClassUnknown

	RoleAnode   = model.RoleAnode
	RoleCathode = model.RoleCathode
	RolePower   = model.RolePower
	RoleGround  = model.RoleGround
	RoleGate    = model.RoleGate
	RoleSource  = model.RoleSource
	RoleDrain   = model.RoleDrain
	RoleUnknown = model.RoleUnknown
)

// irModel is the default implementation of the model.Model contract.
var _ model.Model = (*irModel)(nil)
