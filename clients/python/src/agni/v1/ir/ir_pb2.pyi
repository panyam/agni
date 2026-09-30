from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PinDirection(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PIN_DIRECTION_UNSPECIFIED: _ClassVar[PinDirection]
    PIN_DIRECTION_INPUT: _ClassVar[PinDirection]
    PIN_DIRECTION_OUTPUT: _ClassVar[PinDirection]
    PIN_DIRECTION_INOUT: _ClassVar[PinDirection]
    PIN_DIRECTION_PASSIVE: _ClassVar[PinDirection]
    PIN_DIRECTION_POWER: _ClassVar[PinDirection]
    PIN_DIRECTION_NO_CONNECT: _ClassVar[PinDirection]
    PIN_DIRECTION_POWER_IN: _ClassVar[PinDirection]
    PIN_DIRECTION_POWER_OUT: _ClassVar[PinDirection]

class ClassSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CLASS_SOURCE_UNSPECIFIED: _ClassVar[ClassSource]
    CLASS_SOURCE_CONVENTION: _ClassVar[ClassSource]
    CLASS_SOURCE_DATASHEET: _ClassVar[ClassSource]

class RoleSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ROLE_SOURCE_UNSPECIFIED: _ClassVar[RoleSource]
    ROLE_SOURCE_CONVENTION: _ClassVar[RoleSource]
    ROLE_SOURCE_DECLARED: _ClassVar[RoleSource]
    ROLE_SOURCE_DATASHEET: _ClassVar[RoleSource]

class Role(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ROLE_UNSPECIFIED: _ClassVar[Role]
    ROLE_RAIL: _ClassVar[Role]
    ROLE_GROUND: _ClassVar[Role]
    ROLE_FEEDBACK: _ClassVar[Role]
    ROLE_SWITCHING: _ClassVar[Role]
    ROLE_CONTROL: _ClassVar[Role]
    ROLE_GATE_DRIVE: _ClassVar[Role]

class LayerFunction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LAYER_FUNCTION_UNSPECIFIED: _ClassVar[LayerFunction]
    LAYER_FUNCTION_SIGNAL: _ClassVar[LayerFunction]
    LAYER_FUNCTION_PLANE: _ClassVar[LayerFunction]
    LAYER_FUNCTION_DIELECTRIC: _ClassVar[LayerFunction]
    LAYER_FUNCTION_SOLDER_MASK: _ClassVar[LayerFunction]
    LAYER_FUNCTION_SILKSCREEN: _ClassVar[LayerFunction]
    LAYER_FUNCTION_PASTE: _ClassVar[LayerFunction]
PIN_DIRECTION_UNSPECIFIED: PinDirection
PIN_DIRECTION_INPUT: PinDirection
PIN_DIRECTION_OUTPUT: PinDirection
PIN_DIRECTION_INOUT: PinDirection
PIN_DIRECTION_PASSIVE: PinDirection
PIN_DIRECTION_POWER: PinDirection
PIN_DIRECTION_NO_CONNECT: PinDirection
PIN_DIRECTION_POWER_IN: PinDirection
PIN_DIRECTION_POWER_OUT: PinDirection
CLASS_SOURCE_UNSPECIFIED: ClassSource
CLASS_SOURCE_CONVENTION: ClassSource
CLASS_SOURCE_DATASHEET: ClassSource
ROLE_SOURCE_UNSPECIFIED: RoleSource
ROLE_SOURCE_CONVENTION: RoleSource
ROLE_SOURCE_DECLARED: RoleSource
ROLE_SOURCE_DATASHEET: RoleSource
ROLE_UNSPECIFIED: Role
ROLE_RAIL: Role
ROLE_GROUND: Role
ROLE_FEEDBACK: Role
ROLE_SWITCHING: Role
ROLE_CONTROL: Role
ROLE_GATE_DRIVE: Role
LAYER_FUNCTION_UNSPECIFIED: LayerFunction
LAYER_FUNCTION_SIGNAL: LayerFunction
LAYER_FUNCTION_PLANE: LayerFunction
LAYER_FUNCTION_DIELECTRIC: LayerFunction
LAYER_FUNCTION_SOLDER_MASK: LayerFunction
LAYER_FUNCTION_SILKSCREEN: LayerFunction
LAYER_FUNCTION_PASTE: LayerFunction

class Span(_message.Message):
    __slots__ = ("byte_offset", "byte_length", "line", "column")
    BYTE_OFFSET_FIELD_NUMBER: _ClassVar[int]
    BYTE_LENGTH_FIELD_NUMBER: _ClassVar[int]
    LINE_FIELD_NUMBER: _ClassVar[int]
    COLUMN_FIELD_NUMBER: _ClassVar[int]
    byte_offset: int
    byte_length: int
    line: int
    column: int
    def __init__(self, byte_offset: _Optional[int] = ..., byte_length: _Optional[int] = ..., line: _Optional[int] = ..., column: _Optional[int] = ...) -> None: ...

class Provenance(_message.Message):
    __slots__ = ("source_file", "span", "native_id", "native_id_kind")
    SOURCE_FILE_FIELD_NUMBER: _ClassVar[int]
    SPAN_FIELD_NUMBER: _ClassVar[int]
    NATIVE_ID_FIELD_NUMBER: _ClassVar[int]
    NATIVE_ID_KIND_FIELD_NUMBER: _ClassVar[int]
    source_file: str
    span: Span
    native_id: str
    native_id_kind: str
    def __init__(self, source_file: _Optional[str] = ..., span: _Optional[_Union[Span, _Mapping]] = ..., native_id: _Optional[str] = ..., native_id_kind: _Optional[str] = ...) -> None: ...

class FidelityFragment(_message.Message):
    __slots__ = ("prov", "format", "raw", "note")
    PROV_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    RAW_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    prov: Provenance
    format: str
    raw: bytes
    note: str
    def __init__(self, prov: _Optional[_Union[Provenance, _Mapping]] = ..., format: _Optional[str] = ..., raw: _Optional[bytes] = ..., note: _Optional[str] = ...) -> None: ...

class Design(_message.Message):
    __slots__ = ("name", "ir_version", "source_format", "libraries", "components", "nets", "sheets", "input_diagnostics", "footprints", "layers", "stackup", "constraints", "bom", "attributes", "fidelity", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    IR_VERSION_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    SHEETS_FIELD_NUMBER: _ClassVar[int]
    INPUT_DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    FOOTPRINTS_FIELD_NUMBER: _ClassVar[int]
    LAYERS_FIELD_NUMBER: _ClassVar[int]
    STACKUP_FIELD_NUMBER: _ClassVar[int]
    CONSTRAINTS_FIELD_NUMBER: _ClassVar[int]
    BOM_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    FIDELITY_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    ir_version: str
    source_format: str
    libraries: _containers.RepeatedCompositeFieldContainer[PartLibrary]
    components: _containers.RepeatedCompositeFieldContainer[Component]
    nets: _containers.RepeatedCompositeFieldContainer[Net]
    sheets: _containers.RepeatedCompositeFieldContainer[Sheet]
    input_diagnostics: InputDiagnostics
    footprints: _containers.RepeatedCompositeFieldContainer[Footprint]
    layers: _containers.RepeatedCompositeFieldContainer[Layer]
    stackup: Stackup
    constraints: _containers.RepeatedCompositeFieldContainer[Constraint]
    bom: _containers.RepeatedCompositeFieldContainer[BomLine]
    attributes: _containers.ScalarMap[str, str]
    fidelity: _containers.RepeatedCompositeFieldContainer[FidelityFragment]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., ir_version: _Optional[str] = ..., source_format: _Optional[str] = ..., libraries: _Optional[_Iterable[_Union[PartLibrary, _Mapping]]] = ..., components: _Optional[_Iterable[_Union[Component, _Mapping]]] = ..., nets: _Optional[_Iterable[_Union[Net, _Mapping]]] = ..., sheets: _Optional[_Iterable[_Union[Sheet, _Mapping]]] = ..., input_diagnostics: _Optional[_Union[InputDiagnostics, _Mapping]] = ..., footprints: _Optional[_Iterable[_Union[Footprint, _Mapping]]] = ..., layers: _Optional[_Iterable[_Union[Layer, _Mapping]]] = ..., stackup: _Optional[_Union[Stackup, _Mapping]] = ..., constraints: _Optional[_Iterable[_Union[Constraint, _Mapping]]] = ..., bom: _Optional[_Iterable[_Union[BomLine, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., fidelity: _Optional[_Iterable[_Union[FidelityFragment, _Mapping]]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class InputDiagnostics(_message.Message):
    __slots__ = ("dangling_endpoints", "ref_des_collisions", "no_junction_endpoints", "unmodeled_buses", "unresolved_symbols", "resolved_symbols", "joined_taps", "unannotated_components", "unexpanded_hierarchy", "supplied")
    DANGLING_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    REF_DES_COLLISIONS_FIELD_NUMBER: _ClassVar[int]
    NO_JUNCTION_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    UNMODELED_BUSES_FIELD_NUMBER: _ClassVar[int]
    UNRESOLVED_SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    JOINED_TAPS_FIELD_NUMBER: _ClassVar[int]
    UNANNOTATED_COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    UNEXPANDED_HIERARCHY_FIELD_NUMBER: _ClassVar[int]
    SUPPLIED_FIELD_NUMBER: _ClassVar[int]
    dangling_endpoints: _containers.RepeatedCompositeFieldContainer[DanglingEndpoint]
    ref_des_collisions: _containers.RepeatedCompositeFieldContainer[RefDesCollision]
    no_junction_endpoints: _containers.RepeatedCompositeFieldContainer[DanglingEndpoint]
    unmodeled_buses: _containers.RepeatedCompositeFieldContainer[BusNotModeled]
    unresolved_symbols: _containers.RepeatedCompositeFieldContainer[UnresolvedSymbol]
    resolved_symbols: _containers.RepeatedCompositeFieldContainer[ResolvedSymbol]
    joined_taps: _containers.RepeatedCompositeFieldContainer[JoinedTap]
    unannotated_components: _containers.RepeatedCompositeFieldContainer[UnannotatedComponent]
    unexpanded_hierarchy: _containers.RepeatedCompositeFieldContainer[UnexpandedHierarchy]
    supplied: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, dangling_endpoints: _Optional[_Iterable[_Union[DanglingEndpoint, _Mapping]]] = ..., ref_des_collisions: _Optional[_Iterable[_Union[RefDesCollision, _Mapping]]] = ..., no_junction_endpoints: _Optional[_Iterable[_Union[DanglingEndpoint, _Mapping]]] = ..., unmodeled_buses: _Optional[_Iterable[_Union[BusNotModeled, _Mapping]]] = ..., unresolved_symbols: _Optional[_Iterable[_Union[UnresolvedSymbol, _Mapping]]] = ..., resolved_symbols: _Optional[_Iterable[_Union[ResolvedSymbol, _Mapping]]] = ..., joined_taps: _Optional[_Iterable[_Union[JoinedTap, _Mapping]]] = ..., unannotated_components: _Optional[_Iterable[_Union[UnannotatedComponent, _Mapping]]] = ..., unexpanded_hierarchy: _Optional[_Iterable[_Union[UnexpandedHierarchy, _Mapping]]] = ..., supplied: _Optional[_Iterable[str]] = ...) -> None: ...

class UnexpandedHierarchy(_message.Message):
    __slots__ = ("name", "kind", "instance_count", "prov")
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    INSTANCE_COUNT_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: str
    instance_count: int
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[str] = ..., instance_count: _Optional[int] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class UnannotatedComponent(_message.Message):
    __slots__ = ("ref_des", "instances")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    instances: _containers.RepeatedCompositeFieldContainer[Provenance]
    def __init__(self, ref_des: _Optional[str] = ..., instances: _Optional[_Iterable[_Union[Provenance, _Mapping]]] = ...) -> None: ...

class ResolvedSymbol(_message.Message):
    __slots__ = ("symref", "kind", "pin_count")
    SYMREF_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PIN_COUNT_FIELD_NUMBER: _ClassVar[int]
    symref: str
    kind: str
    pin_count: int
    def __init__(self, symref: _Optional[str] = ..., kind: _Optional[str] = ..., pin_count: _Optional[int] = ...) -> None: ...

class UnresolvedSymbol(_message.Message):
    __slots__ = ("symref", "kind", "ref_des", "prov")
    SYMREF_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    symref: str
    kind: str
    ref_des: _containers.RepeatedScalarFieldContainer[str]
    prov: Provenance
    def __init__(self, symref: _Optional[str] = ..., kind: _Optional[str] = ..., ref_des: _Optional[_Iterable[str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class BusNotModeled(_message.Message):
    __slots__ = ("label", "kind", "prov", "members")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    label: str
    kind: str
    prov: Provenance
    members: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, label: _Optional[str] = ..., kind: _Optional[str] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ..., members: _Optional[_Iterable[str]] = ...) -> None: ...

class JoinedTap(_message.Message):
    __slots__ = ("x", "y", "join_kind", "label", "segments", "prov")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    JOIN_KIND_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    x: int
    y: int
    join_kind: str
    label: str
    segments: int
    prov: Provenance
    def __init__(self, x: _Optional[int] = ..., y: _Optional[int] = ..., join_kind: _Optional[str] = ..., label: _Optional[str] = ..., segments: _Optional[int] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class DanglingEndpoint(_message.Message):
    __slots__ = ("x", "y", "prov")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    x: int
    y: int
    prov: Provenance
    def __init__(self, x: _Optional[int] = ..., y: _Optional[int] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class RefDesCollision(_message.Message):
    __slots__ = ("ref_des", "instances")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    instances: _containers.RepeatedCompositeFieldContainer[Provenance]
    def __init__(self, ref_des: _Optional[str] = ..., instances: _Optional[_Iterable[_Union[Provenance, _Mapping]]] = ...) -> None: ...

class PartLibrary(_message.Message):
    __slots__ = ("name", "parts", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARTS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    parts: _containers.RepeatedCompositeFieldContainer[PartType]
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., parts: _Optional[_Iterable[_Union[PartType, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class PartType(_message.Message):
    __slots__ = ("name", "kind", "designator_prefix", "pins", "mpn", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    DESIGNATOR_PREFIX_FIELD_NUMBER: _ClassVar[int]
    PINS_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: str
    designator_prefix: str
    pins: _containers.RepeatedCompositeFieldContainer[Pin]
    mpn: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[str] = ..., designator_prefix: _Optional[str] = ..., pins: _Optional[_Iterable[_Union[Pin, _Mapping]]] = ..., mpn: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Pin(_message.Message):
    __slots__ = ("name", "designator", "direction", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESIGNATOR_FIELD_NUMBER: _ClassVar[int]
    DIRECTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    designator: str
    direction: PinDirection
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., designator: _Optional[str] = ..., direction: _Optional[_Union[PinDirection, str]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Component(_message.Message):
    __slots__ = ("ref_des", "sections", "footprint_ref", "device_classes", "value", "mpn", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    SECTIONS_FIELD_NUMBER: _ClassVar[int]
    FOOTPRINT_REF_FIELD_NUMBER: _ClassVar[int]
    DEVICE_CLASSES_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    sections: _containers.RepeatedCompositeFieldContainer[ComponentSection]
    footprint_ref: str
    device_classes: _containers.RepeatedCompositeFieldContainer[ComponentClassTag]
    value: Quantity
    mpn: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, ref_des: _Optional[str] = ..., sections: _Optional[_Iterable[_Union[ComponentSection, _Mapping]]] = ..., footprint_ref: _Optional[str] = ..., device_classes: _Optional[_Iterable[_Union[ComponentClassTag, _Mapping]]] = ..., value: _Optional[_Union[Quantity, _Mapping]] = ..., mpn: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class ComponentClassTag(_message.Message):
    __slots__ = ("source",)
    CLASS_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    source: ClassSource
    def __init__(self, source: _Optional[_Union[ClassSource, str]] = ..., **kwargs) -> None: ...

class Quantity(_message.Message):
    __slots__ = ("input", "value", "unit")
    INPUT_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    input: str
    value: float
    unit: str
    def __init__(self, input: _Optional[str] = ..., value: _Optional[float] = ..., unit: _Optional[str] = ...) -> None: ...

class ComponentSection(_message.Message):
    __slots__ = ("index", "part_ref", "library_ref", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    INDEX_FIELD_NUMBER: _ClassVar[int]
    PART_REF_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_REF_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    index: int
    part_ref: str
    library_ref: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, index: _Optional[int] = ..., part_ref: _Optional[str] = ..., library_ref: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class NetRole(_message.Message):
    __slots__ = ("role_kind", "source")
    ROLE_KIND_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    role_kind: Role
    source: RoleSource
    def __init__(self, role_kind: _Optional[_Union[Role, str]] = ..., source: _Optional[_Union[RoleSource, str]] = ...) -> None: ...

class Net(_message.Message):
    __slots__ = ("name", "net_classes", "connections", "id", "roles", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    NET_CLASSES_FIELD_NUMBER: _ClassVar[int]
    CONNECTIONS_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    net_classes: _containers.RepeatedScalarFieldContainer[str]
    connections: _containers.RepeatedCompositeFieldContainer[Connection]
    id: str
    roles: _containers.RepeatedCompositeFieldContainer[NetRole]
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., net_classes: _Optional[_Iterable[str]] = ..., connections: _Optional[_Iterable[_Union[Connection, _Mapping]]] = ..., id: _Optional[str] = ..., roles: _Optional[_Iterable[_Union[NetRole, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Connection(_message.Message):
    __slots__ = ("component_ref", "pin_ref", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    COMPONENT_REF_FIELD_NUMBER: _ClassVar[int]
    PIN_REF_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    component_ref: str
    pin_ref: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, component_ref: _Optional[str] = ..., pin_ref: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Sheet(_message.Message):
    __slots__ = ("id", "name", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Footprint(_message.Message):
    __slots__ = ("name", "library", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    library: str
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., library: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Layer(_message.Message):
    __slots__ = ("name", "index", "function", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    index: int
    function: LayerFunction
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., index: _Optional[int] = ..., function: _Optional[_Union[LayerFunction, str]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class StackupLayer(_message.Message):
    __slots__ = ("layer_ref", "thickness_nm", "material", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LAYER_REF_FIELD_NUMBER: _ClassVar[int]
    THICKNESS_NM_FIELD_NUMBER: _ClassVar[int]
    MATERIAL_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    layer_ref: str
    thickness_nm: int
    material: str
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, layer_ref: _Optional[str] = ..., thickness_nm: _Optional[int] = ..., material: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Stackup(_message.Message):
    __slots__ = ("layers", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LAYERS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    layers: _containers.RepeatedCompositeFieldContainer[StackupLayer]
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, layers: _Optional[_Iterable[_Union[StackupLayer, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Constraint(_message.Message):
    __slots__ = ("name", "kind", "params", "attributes", "prov")
    class ParamsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: str
    params: _containers.ScalarMap[str, str]
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[str] = ..., params: _Optional[_Mapping[str, str]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class BomLine(_message.Message):
    __slots__ = ("ref_des", "mpn", "manufacturer", "quantity", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    MANUFACTURER_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    ref_des: _containers.RepeatedScalarFieldContainer[str]
    mpn: str
    manufacturer: str
    quantity: int
    attributes: _containers.ScalarMap[str, str]
    prov: Provenance
    def __init__(self, ref_des: _Optional[_Iterable[str]] = ..., mpn: _Optional[str] = ..., manufacturer: _Optional[str] = ..., quantity: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...
