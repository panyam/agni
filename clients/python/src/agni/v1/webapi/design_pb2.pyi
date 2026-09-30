from agni.v1.geom import geom_pb2 as _geom_pb2
from agni.v1.geom import geom_packed_pb2 as _geom_packed_pb2
from agni.v1.ir import ir_pb2 as _ir_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SheetFormat(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SHEET_FORMAT_UNSPECIFIED: _ClassVar[SheetFormat]
    SHEET_FORMAT_PACKED: _ClassVar[SheetFormat]
    SHEET_FORMAT_SVG: _ClassVar[SheetFormat]
    SHEET_FORMAT_NATIVE: _ClassVar[SheetFormat]

class SymbolSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SYMBOL_SOURCE_UNSPECIFIED: _ClassVar[SymbolSource]
    SYMBOL_SOURCE_GLYPH: _ClassVar[SymbolSource]
    SYMBOL_SOURCE_FAITHFUL: _ClassVar[SymbolSource]

class TraceOutcome(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRACE_OUTCOME_UNSPECIFIED: _ClassVar[TraceOutcome]
    TRACE_OUTCOME_ROUTED: _ClassVar[TraceOutcome]
    TRACE_OUTCOME_NO_ROUTE: _ClassVar[TraceOutcome]
    TRACE_OUTCOME_UNRESOLVED: _ClassVar[TraceOutcome]
SHEET_FORMAT_UNSPECIFIED: SheetFormat
SHEET_FORMAT_PACKED: SheetFormat
SHEET_FORMAT_SVG: SheetFormat
SHEET_FORMAT_NATIVE: SheetFormat
SYMBOL_SOURCE_UNSPECIFIED: SymbolSource
SYMBOL_SOURCE_GLYPH: SymbolSource
SYMBOL_SOURCE_FAITHFUL: SymbolSource
TRACE_OUTCOME_UNSPECIFIED: TraceOutcome
TRACE_OUTCOME_ROUTED: TraceOutcome
TRACE_OUTCOME_NO_ROUTE: TraceOutcome
TRACE_OUTCOME_UNRESOLVED: TraceOutcome

class SheetRef(_message.Message):
    __slots__ = ("id", "name", "parent_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARENT_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    parent_id: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., parent_id: _Optional[str] = ...) -> None: ...

class GetDesignRequest(_message.Message):
    __slots__ = ("layout", "uri", "as_named")
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    layout: str
    uri: str
    as_named: bool
    def __init__(self, layout: _Optional[str] = ..., uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class GetDesignResponse(_message.Message):
    __slots__ = ("name", "source_format", "component_count", "net_count", "undrawn", "layout", "sheets", "native_available", "available_layouts", "content_hash", "unexpanded_hierarchy")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_COUNT_FIELD_NUMBER: _ClassVar[int]
    NET_COUNT_FIELD_NUMBER: _ClassVar[int]
    UNDRAWN_FIELD_NUMBER: _ClassVar[int]
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    SHEETS_FIELD_NUMBER: _ClassVar[int]
    NATIVE_AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_LAYOUTS_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    UNEXPANDED_HIERARCHY_FIELD_NUMBER: _ClassVar[int]
    name: str
    source_format: str
    component_count: int
    net_count: int
    undrawn: _containers.RepeatedCompositeFieldContainer[_geom_pb2.UndrawnPlacement]
    layout: str
    sheets: _containers.RepeatedCompositeFieldContainer[SheetRef]
    native_available: bool
    available_layouts: _containers.RepeatedScalarFieldContainer[str]
    content_hash: str
    unexpanded_hierarchy: _containers.RepeatedCompositeFieldContainer[_ir_pb2.UnexpandedHierarchy]
    def __init__(self, name: _Optional[str] = ..., source_format: _Optional[str] = ..., component_count: _Optional[int] = ..., net_count: _Optional[int] = ..., undrawn: _Optional[_Iterable[_Union[_geom_pb2.UndrawnPlacement, _Mapping]]] = ..., layout: _Optional[str] = ..., sheets: _Optional[_Iterable[_Union[SheetRef, _Mapping]]] = ..., native_available: _Optional[bool] = ..., available_layouts: _Optional[_Iterable[str]] = ..., content_hash: _Optional[str] = ..., unexpanded_hierarchy: _Optional[_Iterable[_Union[_ir_pb2.UnexpandedHierarchy, _Mapping]]] = ...) -> None: ...

class GetSheetRequest(_message.Message):
    __slots__ = ("sheet", "layout", "format", "symbols", "uri", "as_named")
    SHEET_FIELD_NUMBER: _ClassVar[int]
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    sheet: str
    layout: str
    format: SheetFormat
    symbols: SymbolSource
    uri: str
    as_named: bool
    def __init__(self, sheet: _Optional[str] = ..., layout: _Optional[str] = ..., format: _Optional[_Union[SheetFormat, str]] = ..., symbols: _Optional[_Union[SymbolSource, str]] = ..., uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class GetSheetResponse(_message.Message):
    __slots__ = ("packed", "svg")
    PACKED_FIELD_NUMBER: _ClassVar[int]
    SVG_FIELD_NUMBER: _ClassVar[int]
    packed: _geom_packed_pb2.PackedSheet
    svg: str
    def __init__(self, packed: _Optional[_Union[_geom_packed_pb2.PackedSheet, _Mapping]] = ..., svg: _Optional[str] = ...) -> None: ...

class HighlightSheetRequest(_message.Message):
    __slots__ = ("sheet", "layout", "symbols", "format", "specs", "uri", "as_named")
    SHEET_FIELD_NUMBER: _ClassVar[int]
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    SPECS_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    sheet: str
    layout: str
    symbols: SymbolSource
    format: SheetFormat
    specs: _containers.RepeatedCompositeFieldContainer[_geom_packed_pb2.HighlightSpec]
    uri: str
    as_named: bool
    def __init__(self, sheet: _Optional[str] = ..., layout: _Optional[str] = ..., symbols: _Optional[_Union[SymbolSource, str]] = ..., format: _Optional[_Union[SheetFormat, str]] = ..., specs: _Optional[_Iterable[_Union[_geom_packed_pb2.HighlightSpec, _Mapping]]] = ..., uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class HighlightSheetResponse(_message.Message):
    __slots__ = ("packed", "svg")
    PACKED_FIELD_NUMBER: _ClassVar[int]
    SVG_FIELD_NUMBER: _ClassVar[int]
    packed: _geom_packed_pb2.PackedHighlight
    svg: str
    def __init__(self, packed: _Optional[_Union[_geom_packed_pb2.PackedHighlight, _Mapping]] = ..., svg: _Optional[str] = ...) -> None: ...

class GetLayoutReportRequest(_message.Message):
    __slots__ = ("symbols", "uri", "as_named")
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    symbols: SymbolSource
    uri: str
    as_named: bool
    def __init__(self, symbols: _Optional[_Union[SymbolSource, str]] = ..., uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class GetLayoutReportResponse(_message.Message):
    __slots__ = ("report",)
    REPORT_FIELD_NUMBER: _ClassVar[int]
    report: ConversionReport
    def __init__(self, report: _Optional[_Union[ConversionReport, _Mapping]] = ...) -> None: ...

class TraceDesignRequest(_message.Message):
    __slots__ = ("uri", "to", "hops", "as_named")
    URI_FIELD_NUMBER: _ClassVar[int]
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    HOPS_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    uri: str
    to: TraceEndpoint
    hops: int
    as_named: bool
    def __init__(self, uri: _Optional[str] = ..., to: _Optional[_Union[TraceEndpoint, _Mapping]] = ..., hops: _Optional[int] = ..., as_named: _Optional[bool] = ..., **kwargs) -> None: ...

class TraceEndpoint(_message.Message):
    __slots__ = ("ref_des", "pin")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    pin: str
    def __init__(self, ref_des: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...

class TraceDesignResponse(_message.Message):
    __slots__ = ("trace",)
    TRACE_FIELD_NUMBER: _ClassVar[int]
    trace: Trace
    def __init__(self, trace: _Optional[_Union[Trace, _Mapping]] = ...) -> None: ...

class Trace(_message.Message):
    __slots__ = ("to", "outcome", "reason", "radius", "crossings", "nets")
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    OUTCOME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    RADIUS_FIELD_NUMBER: _ClassVar[int]
    CROSSINGS_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    to: TraceEnd
    outcome: TraceOutcome
    reason: str
    radius: int
    crossings: _containers.RepeatedCompositeFieldContainer[TraceCross]
    nets: _containers.RepeatedCompositeFieldContainer[TraceNet]
    def __init__(self, to: _Optional[_Union[TraceEnd, _Mapping]] = ..., outcome: _Optional[_Union[TraceOutcome, str]] = ..., reason: _Optional[str] = ..., radius: _Optional[int] = ..., crossings: _Optional[_Iterable[_Union[TraceCross, _Mapping]]] = ..., nets: _Optional[_Iterable[_Union[TraceNet, _Mapping]]] = ..., **kwargs) -> None: ...

class TraceEnd(_message.Message):
    __slots__ = ("endpoint", "pin_name", "net", "sheet_ids")
    ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    PIN_NAME_FIELD_NUMBER: _ClassVar[int]
    NET_FIELD_NUMBER: _ClassVar[int]
    SHEET_IDS_FIELD_NUMBER: _ClassVar[int]
    endpoint: TraceEndpoint
    pin_name: str
    net: str
    sheet_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, endpoint: _Optional[_Union[TraceEndpoint, _Mapping]] = ..., pin_name: _Optional[str] = ..., net: _Optional[str] = ..., sheet_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class TraceCross(_message.Message):
    __slots__ = ("ref_des", "enter_pin", "exit_pin", "from_net", "to_net")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    ENTER_PIN_FIELD_NUMBER: _ClassVar[int]
    EXIT_PIN_FIELD_NUMBER: _ClassVar[int]
    FROM_NET_FIELD_NUMBER: _ClassVar[int]
    TO_NET_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    enter_pin: str
    exit_pin: str
    from_net: str
    to_net: str
    def __init__(self, ref_des: _Optional[str] = ..., enter_pin: _Optional[str] = ..., exit_pin: _Optional[str] = ..., from_net: _Optional[str] = ..., to_net: _Optional[str] = ..., **kwargs) -> None: ...

class TraceNet(_message.Message):
    __slots__ = ("name", "stubs", "stubs_elided", "bus_like", "sheet_ids")
    NAME_FIELD_NUMBER: _ClassVar[int]
    STUBS_FIELD_NUMBER: _ClassVar[int]
    STUBS_ELIDED_FIELD_NUMBER: _ClassVar[int]
    BUS_LIKE_FIELD_NUMBER: _ClassVar[int]
    SHEET_IDS_FIELD_NUMBER: _ClassVar[int]
    name: str
    stubs: _containers.RepeatedCompositeFieldContainer[TraceStub]
    stubs_elided: int
    bus_like: bool
    sheet_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., stubs: _Optional[_Iterable[_Union[TraceStub, _Mapping]]] = ..., stubs_elided: _Optional[int] = ..., bus_like: _Optional[bool] = ..., sheet_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class TraceStub(_message.Message):
    __slots__ = ("ref_des", "pin")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    pin: str
    def __init__(self, ref_des: _Optional[str] = ..., pin: _Optional[str] = ..., **kwargs) -> None: ...

class ConversionReport(_message.Message):
    __slots__ = ("components",)
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedCompositeFieldContainer[ComponentReport]
    def __init__(self, components: _Optional[_Iterable[_Union[ComponentReport, _Mapping]]] = ...) -> None: ...

class ComponentReport(_message.Message):
    __slots__ = ("ref_des", "symbol", "device_class", "cell", "kind")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    DEVICE_CLASS_FIELD_NUMBER: _ClassVar[int]
    CELL_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    symbol: str
    device_class: str
    cell: str
    kind: str
    def __init__(self, ref_des: _Optional[str] = ..., symbol: _Optional[str] = ..., device_class: _Optional[str] = ..., cell: _Optional[str] = ..., kind: _Optional[str] = ...) -> None: ...
