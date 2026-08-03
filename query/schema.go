package query

import "github.com/panyam/agni/check"

// edbField names a FactRow field a relation's positional argument binds to.
type edbField int

const (
	fSubject edbField = iota
	fObject
	fValue
	fNum
	fConditions
	fMin // the SECOND numeric slot (FactRow.Min) — a two-sided relation's lower bound (param.range)
)

// edbSchema maps each fact-base relation to its positional argument layout, so a flat
// check.FactRow is queried as reln(arg0, arg1, ...). This is the ONLY place the fact tuple's shape
// is named; adding an EDB relation is one entry here plus its WS3-004 projector. Relations the
// evaluator computes rather than looks up (reaches) are NOT here.
var edbSchema = map[string][]edbField{
	check.RelNetMaxVoltage:     {fSubject, fNum},                               // net.max_voltage(net, volts)
	check.RelNetNominalVoltage: {fSubject, fNum},                               // net.nominal_voltage(net, volts)
	check.RelComponentMPN:      {fSubject, fValue},                             // component.mpn(ref, mpn)
	check.RelParam:             {fSubject, fObject, fNum},                      // param(mpn, symbol, max)
	check.RelParamRange:        {fSubject, fObject, fValue, fMin, fNum},        // param.range(mpn, symbol, kind, min, max)
	check.RelParamProv:         {fSubject, fObject, fValue, fNum, fConditions}, // param.prov(mpn, symbol, doc, page, section)
	check.RelPartAudience:      {fSubject, fObject},                            // part.audience(mpn, who)
	check.RelComponentOnNet:    {fSubject, fObject},                            // component-on-net(ref, net)
	// Pin tier (WS3-038) — pin-granular relations, queryable with no evaluator change.
	check.RelPin:           {fSubject, fObject},         // pin(ref, pin)
	check.RelPinRole:       {fSubject, fObject, fValue}, // pin.role(ref, pin, role)
	check.RelPinType:       {fSubject, fObject, fValue}, // pin.type(ref, pin, etype)
	check.RelPinNet:        {fSubject, fObject, fValue}, // pin.net(ref, pin, net)
	check.RelNetPinCount:   {fSubject, fNum},            // net.pin_count(net, count)
	check.RelHasNCChannel:  {fSubject},                  // has_nc_channel(present)
	check.RelTypesPowerOut: {fSubject},                  // types_power_out(present)
	check.RelRail:          {fSubject},                  // rail(net)
	check.RelFeedback:      {fSubject},                  // feedback(net)
	check.RelComponentAttr: {fSubject, fObject, fValue}, // component.attr(ref, key, value)
	// Device-class and net-attribute relations (WS3-074). component.class emits one row per class
	// tag in the device_classes SET (WS3-071), so a family tag answers too.
	check.RelComponentClass:       {fSubject, fValue},          // component.class(ref, class)
	check.RelNetGround:            {fSubject},                  // net.ground(net)
	check.RelNetExternal:          {fSubject},                  // net.external(net)
	check.RelEsdRated:             {fSubject},                  // component.esd_rated(ref) — WS3-076, datasheet tier
	check.RelComponentDeviceClass: {fSubject, fValue},          // component.device_class(ref, class) — WS10-013, datasheet tier
	check.RelBus:                  {fSubject, fValue},          // bus(label, kind) — reader-detected unmodeled bus (WS1-034)
	check.RelRefDesCollision:      {fSubject},                  // ref_des_collision(ref_des) — WS3-081
	check.RelPinNetConflict:       {fSubject, fObject, fValue}, // pin_net_conflict(ref_des, pin, net) — WS3-081
	check.RelNetBusLike:           {fSubject},                  // net.bus_like(net) — WS3-080
	// Board tier — queryable with no evaluator change (tier-generality).
	check.RelBoardTrackWidth: {fSubject, fNum},    // board.track_width(net, mm)
	check.RelBoardViaDrill:   {fSubject, fNum},    // board.via_drill(net, mm)
	check.RelBoardLayer:      {fSubject, fObject}, // board.layer(net, layer)
}

// fieldValue reads one FactRow field as a query Value (string + optional number). The numeric
// field carries both so a bound term serves equality and comparison alike.
func fieldValue(f check.FactRow, fld edbField) Value {
	switch fld {
	case fSubject:
		return Value{S: f.Subject}
	case fObject:
		return Value{S: f.Object}
	case fValue:
		return Value{S: f.Value}
	case fNum:
		if f.Num != nil {
			return Value{S: ftoa(*f.Num), Num: f.Num}
		}
		return Value{}
	case fConditions:
		return Value{S: f.Conditions}
	case fMin:
		if f.Min != nil {
			return Value{S: ftoa(*f.Min), Num: f.Min}
		}
		return Value{}
	}
	return Value{}
}
