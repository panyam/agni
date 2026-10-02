package intent

import (
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// fullIntent sets every field of DesignIntent and of each message under it to a value that is
// distinguishable from its zero, in a declaration that still validates. A field left at zero would
// pass any check of the conversion, dropped or not.
func fullIntent() *configpb.DesignIntent {
	return &configpb.DesignIntent{
		Modules: []*configpb.IntentModule{
			{Name: "pmics", Class: "regulator", Mpn: "TPS1", Count: 3},
			{Name: "clock", Class: "crystal", Mpn: "XT1", Nets: []string{"XIN", "XOUT"}},
		},
		Nets: map[string]*configpb.NetIntent{
			"VDD": {
				Nominal: proto.Float64(3.3), Domain: "io", Peak: proto.Float64(0.8),
				Protect: []string{"ovp", "discharge"}, Reset_: "low", Strap: "high",
				MinOhms: 1000, MaxOhms: 47000, AcCoupled: true,
			},
		},
		Sequences: []*configpb.PowerSequence{{
			Name: "soc", Relation: SequenceEnableGated,
			Order: []*configpb.SequenceStage{
				{Rail: "VA", Good: "VA_PG", Enable: "VA_EN"},
				{Rail: "VB", Good: "VB_PG", Enable: "VB_EN"},
			},
		}},
		StrapGroups: []*configpb.StrapGroup{{
			Name: "phyad", Device: "U12", Nets: []string{"A1", "A0"}, Value: 2, Bus: "MDIO", Default: "low",
		}},
		IoMap: []*configpb.PinAssignment{{
			Net: "SDA", Device: "U3", Pin: "9", Function: "I2C0_SDA",
			To: &configpb.PinEndpoint{Device: "U1", Pin: "5"},
		}},
		MarginFactor: 1.2,
	}
}

// TestFromProtoCarriesEveryField holds FromProto to the schema. Each field of the message tree has a
// check that its fixture value reached the Declaration, and the test walks the DESCRIPTORS, so a field
// added to intent.proto fails here until somebody decides where it goes. That is the guard C26 asks of
// a converter, in the one-way form a compile step allows.
func TestFromProtoCarriesEveryField(t *testing.T) {
	d, err := FromProto("board", fullIntent())
	if err != nil {
		t.Fatal(err)
	}
	var vdd NetProperty
	var strap NetProperty
	for _, p := range d.NetProperties {
		switch p.Property {
		case PropResetPolarity:
			vdd = p
		case PropStrap:
			strap = p
		}
	}
	checks := map[protoreflect.FullName]func() bool{
		"agni.v1.config.DesignIntent.modules":       func() bool { return len(d.Modules) == 1 && len(d.Subsystems) == 1 },
		"agni.v1.config.DesignIntent.nets":          func() bool { return len(d.VoltageDomains) == 1 },
		"agni.v1.config.DesignIntent.sequences":     func() bool { return len(d.Sequences) == 1 },
		"agni.v1.config.DesignIntent.strap_groups":  func() bool { return len(d.StrapGroups) == 1 },
		"agni.v1.config.DesignIntent.io_map":        func() bool { return len(d.IOMap) == 1 },
		"agni.v1.config.DesignIntent.margin_factor": func() bool { return d.MarginFactor == 1.2 },

		"agni.v1.config.IntentModule.name":  func() bool { return d.Modules[0].Name == "pmics" && d.Subsystems[0].Name == "clock" },
		"agni.v1.config.IntentModule.class": func() bool { return d.Modules[0].Class == "regulator" && d.Subsystems[0].Source.Class == "crystal" },
		"agni.v1.config.IntentModule.mpn":   func() bool { return d.Modules[0].MPN == "TPS1" && d.Subsystems[0].Source.MPN == "XT1" },
		"agni.v1.config.IntentModule.count": func() bool { return d.Modules[0].Count == 3 },
		"agni.v1.config.IntentModule.nets":  func() bool { return len(d.Subsystems[0].Nets) == 2 && d.Subsystems[0].Nets[1] == "XOUT" },

		"agni.v1.config.NetIntent.nominal": func() bool { return d.VoltageDomains[0].Nominal == 3.3 && d.VoltageDomains[0].Rails[0] == "VDD" },
		"agni.v1.config.NetIntent.domain":  func() bool { return d.VoltageDomains[0].Name == "io" },
		"agni.v1.config.NetIntent.peak":    func() bool { return len(d.RailBudgets) == 1 && d.RailBudgets[0].Peak == 0.8 },
		"agni.v1.config.NetIntent.protect": func() bool {
			return len(d.Protections) == 2 && d.Protections[1].Kind == ProtectionDischarge
		},
		"agni.v1.config.NetIntent.reset":      func() bool { return vdd.Net == "VDD" && vdd.Value == "low" },
		"agni.v1.config.NetIntent.strap":      func() bool { return strap.Value == "high" },
		"agni.v1.config.NetIntent.min_ohms":   func() bool { return strap.MinOhms == 1000 },
		"agni.v1.config.NetIntent.max_ohms":   func() bool { return strap.MaxOhms == 47000 },
		"agni.v1.config.NetIntent.ac_coupled": func() bool { return len(d.NetProperties) == 3 },

		"agni.v1.config.PowerSequence.name":     func() bool { return d.Sequences[0].Name == "soc" },
		"agni.v1.config.PowerSequence.relation": func() bool { return d.Sequences[0].Relation == SequenceEnableGated },
		"agni.v1.config.PowerSequence.order":    func() bool { return len(d.Sequences[0].Order) == 2 },
		"agni.v1.config.SequenceStage.rail":     func() bool { return d.Sequences[0].Order[1].Rail == "VB" },
		"agni.v1.config.SequenceStage.good":     func() bool { return d.Sequences[0].Order[1].Good == "VB_PG" },
		"agni.v1.config.SequenceStage.enable":   func() bool { return d.Sequences[0].Order[1].Enable == "VB_EN" },

		"agni.v1.config.StrapGroup.name":    func() bool { return d.StrapGroups[0].Name == "phyad" },
		"agni.v1.config.StrapGroup.device":  func() bool { return d.StrapGroups[0].Device == "U12" },
		"agni.v1.config.StrapGroup.nets":    func() bool { return len(d.StrapGroups[0].Nets) == 2 && d.StrapGroups[0].Nets[1] == "A0" },
		"agni.v1.config.StrapGroup.value":   func() bool { return d.StrapGroups[0].Value == 2 },
		"agni.v1.config.StrapGroup.bus":     func() bool { return d.StrapGroups[0].Bus == "MDIO" },
		"agni.v1.config.StrapGroup.default": func() bool { return d.StrapGroups[0].Default == "low" },

		"agni.v1.config.PinAssignment.net":      func() bool { return d.IOMap[0].Net == "SDA" },
		"agni.v1.config.PinAssignment.device":   func() bool { return d.IOMap[0].Device == "U3" },
		"agni.v1.config.PinAssignment.pin":      func() bool { return d.IOMap[0].Pin == "9" },
		"agni.v1.config.PinAssignment.function": func() bool { return d.IOMap[0].Function == "I2C0_SDA" },
		"agni.v1.config.PinAssignment.to":       func() bool { return d.IOMap[0].To != nil },
		"agni.v1.config.PinEndpoint.device":     func() bool { return d.IOMap[0].To.Device == "U1" },
		"agni.v1.config.PinEndpoint.pin":        func() bool { return d.IOMap[0].To.Pin == "5" },
	}

	seen := map[protoreflect.FullName]bool{}
	var walk func(md protoreflect.MessageDescriptor)
	walk = func(md protoreflect.MessageDescriptor) {
		fs := md.Fields()
		for i := 0; i < fs.Len(); i++ {
			fd := fs.Get(i)
			if seen[fd.FullName()] {
				continue
			}
			seen[fd.FullName()] = true
			check, ok := checks[fd.FullName()]
			switch {
			case !ok:
				t.Errorf("%s has no check that it reaches the Declaration; add one here and a value to fullIntent", fd.FullName())
			case !check():
				t.Errorf("%s did not reach the Declaration", fd.FullName())
			}
			if fd.IsMap() {
				fd = fd.MapValue()
			}
			if fd.Message() != nil {
				walk(fd.Message())
			}
		}
	}
	walk((&configpb.DesignIntent{}).ProtoReflect().Descriptor())
	// A positive control on the walk itself, so a descriptor walk that visits nothing cannot pass.
	if len(seen) != len(checks) {
		t.Errorf("walked %d fields against %d checks", len(seen), len(checks))
	}
}
