from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Provenance(_message.Message):
    __slots__ = ("source_file", "source_id")
    SOURCE_FILE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    source_file: str
    source_id: str
    def __init__(self, source_file: _Optional[str] = ..., source_id: _Optional[str] = ...) -> None: ...

class Point(_message.Message):
    __slots__ = ("x", "y")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    x: int
    y: int
    def __init__(self, x: _Optional[int] = ..., y: _Optional[int] = ...) -> None: ...

class BBox(_message.Message):
    __slots__ = ("min", "max")
    MIN_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    min: Point
    max: Point
    def __init__(self, min: _Optional[_Union[Point, _Mapping]] = ..., max: _Optional[_Union[Point, _Mapping]] = ...) -> None: ...

class Transform(_message.Message):
    __slots__ = ("origin", "rotation_deg", "mirror_x", "mirror_y", "scale_x", "scale_y")
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    MIRROR_X_FIELD_NUMBER: _ClassVar[int]
    MIRROR_Y_FIELD_NUMBER: _ClassVar[int]
    SCALE_X_FIELD_NUMBER: _ClassVar[int]
    SCALE_Y_FIELD_NUMBER: _ClassVar[int]
    origin: Point
    rotation_deg: int
    mirror_x: bool
    mirror_y: bool
    scale_x: float
    scale_y: float
    def __init__(self, origin: _Optional[_Union[Point, _Mapping]] = ..., rotation_deg: _Optional[int] = ..., mirror_x: _Optional[bool] = ..., mirror_y: _Optional[bool] = ..., scale_x: _Optional[float] = ..., scale_y: _Optional[float] = ...) -> None: ...

class Shape(_message.Message):
    __slots__ = ("kind", "points", "radius", "figure_group", "fill", "fill_color")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[Shape.Kind]
        KIND_POLYLINE: _ClassVar[Shape.Kind]
        KIND_RECT: _ClassVar[Shape.Kind]
        KIND_CIRCLE: _ClassVar[Shape.Kind]
        KIND_ARC: _ClassVar[Shape.Kind]
        KIND_DOT: _ClassVar[Shape.Kind]
    KIND_UNSPECIFIED: Shape.Kind
    KIND_POLYLINE: Shape.Kind
    KIND_RECT: Shape.Kind
    KIND_CIRCLE: Shape.Kind
    KIND_ARC: Shape.Kind
    KIND_DOT: Shape.Kind
    class Fill(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        FILL_UNSPECIFIED: _ClassVar[Shape.Fill]
        FILL_OUTLINE: _ClassVar[Shape.Fill]
        FILL_BACKGROUND: _ClassVar[Shape.Fill]
        FILL_COLOR: _ClassVar[Shape.Fill]
    FILL_UNSPECIFIED: Shape.Fill
    FILL_OUTLINE: Shape.Fill
    FILL_BACKGROUND: Shape.Fill
    FILL_COLOR: Shape.Fill
    KIND_FIELD_NUMBER: _ClassVar[int]
    POINTS_FIELD_NUMBER: _ClassVar[int]
    RADIUS_FIELD_NUMBER: _ClassVar[int]
    FIGURE_GROUP_FIELD_NUMBER: _ClassVar[int]
    FILL_FIELD_NUMBER: _ClassVar[int]
    FILL_COLOR_FIELD_NUMBER: _ClassVar[int]
    kind: Shape.Kind
    points: _containers.RepeatedCompositeFieldContainer[Point]
    radius: int
    figure_group: str
    fill: Shape.Fill
    fill_color: str
    def __init__(self, kind: _Optional[_Union[Shape.Kind, str]] = ..., points: _Optional[_Iterable[_Union[Point, _Mapping]]] = ..., radius: _Optional[int] = ..., figure_group: _Optional[str] = ..., fill: _Optional[_Union[Shape.Fill, str]] = ..., fill_color: _Optional[str] = ...) -> None: ...

class PinPoint(_message.Message):
    __slots__ = ("port_ref", "loc", "source_id", "label_origin", "justify", "name", "height", "number_origin", "number_justify")
    PORT_REF_FIELD_NUMBER: _ClassVar[int]
    LOC_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_ORIGIN_FIELD_NUMBER: _ClassVar[int]
    JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    NUMBER_ORIGIN_FIELD_NUMBER: _ClassVar[int]
    NUMBER_JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    port_ref: str
    loc: Point
    source_id: str
    label_origin: Point
    justify: str
    name: str
    height: int
    number_origin: Point
    number_justify: str
    def __init__(self, port_ref: _Optional[str] = ..., loc: _Optional[_Union[Point, _Mapping]] = ..., source_id: _Optional[str] = ..., label_origin: _Optional[_Union[Point, _Mapping]] = ..., justify: _Optional[str] = ..., name: _Optional[str] = ..., height: _Optional[int] = ..., number_origin: _Optional[_Union[Point, _Mapping]] = ..., number_justify: _Optional[str] = ...) -> None: ...

class Asset(_message.Message):
    __slots__ = ("kind", "id", "prov", "placeholder")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[Asset.Kind]
        KIND_SYMBOL: _ClassVar[Asset.Kind]
        KIND_IMAGE: _ClassVar[Asset.Kind]
        KIND_GROUP: _ClassVar[Asset.Kind]
    KIND_UNSPECIFIED: Asset.Kind
    KIND_SYMBOL: Asset.Kind
    KIND_IMAGE: Asset.Kind
    KIND_GROUP: Asset.Kind
    KIND_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    PLACEHOLDER_FIELD_NUMBER: _ClassVar[int]
    kind: Asset.Kind
    id: str
    prov: Provenance
    placeholder: bool
    def __init__(self, kind: _Optional[_Union[Asset.Kind, str]] = ..., id: _Optional[str] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ..., placeholder: _Optional[bool] = ...) -> None: ...

class Image(_message.Message):
    __slots__ = ("bbox", "mime", "data", "rotation_deg", "mirror", "asset")
    BBOX_FIELD_NUMBER: _ClassVar[int]
    MIME_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    MIRROR_FIELD_NUMBER: _ClassVar[int]
    ASSET_FIELD_NUMBER: _ClassVar[int]
    bbox: BBox
    mime: str
    data: bytes
    rotation_deg: int
    mirror: bool
    asset: Asset
    def __init__(self, bbox: _Optional[_Union[BBox, _Mapping]] = ..., mime: _Optional[str] = ..., data: _Optional[bytes] = ..., rotation_deg: _Optional[int] = ..., mirror: _Optional[bool] = ..., asset: _Optional[_Union[Asset, _Mapping]] = ...) -> None: ...

class SymbolDef(_message.Message):
    __slots__ = ("cell_ref", "library_ref", "bbox", "shapes", "pins", "view_ref", "annotations", "asset", "images", "prov")
    CELL_REF_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_REF_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    SHAPES_FIELD_NUMBER: _ClassVar[int]
    PINS_FIELD_NUMBER: _ClassVar[int]
    VIEW_REF_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    ASSET_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    cell_ref: str
    library_ref: str
    bbox: BBox
    shapes: _containers.RepeatedCompositeFieldContainer[Shape]
    pins: _containers.RepeatedCompositeFieldContainer[PinPoint]
    view_ref: str
    annotations: _containers.RepeatedCompositeFieldContainer[Label]
    asset: Asset
    images: _containers.RepeatedCompositeFieldContainer[Image]
    prov: Provenance
    def __init__(self, cell_ref: _Optional[str] = ..., library_ref: _Optional[str] = ..., bbox: _Optional[_Union[BBox, _Mapping]] = ..., shapes: _Optional[_Iterable[_Union[Shape, _Mapping]]] = ..., pins: _Optional[_Iterable[_Union[PinPoint, _Mapping]]] = ..., view_ref: _Optional[str] = ..., annotations: _Optional[_Iterable[_Union[Label, _Mapping]]] = ..., asset: _Optional[_Union[Asset, _Mapping]] = ..., images: _Optional[_Iterable[_Union[Image, _Mapping]]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Field(_message.Message):
    __slots__ = ("name", "value", "origin", "justify", "height", "rotation_deg", "visible")
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    VISIBLE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: str
    origin: Point
    justify: str
    height: int
    rotation_deg: int
    visible: bool
    def __init__(self, name: _Optional[str] = ..., value: _Optional[str] = ..., origin: _Optional[_Union[Point, _Mapping]] = ..., justify: _Optional[str] = ..., height: _Optional[int] = ..., rotation_deg: _Optional[int] = ..., visible: _Optional[bool] = ...) -> None: ...

class SymbolPlacement(_message.Message):
    __slots__ = ("ref_des", "cell_ref", "library_ref", "transform", "view_ref", "fields", "net_anchor", "prov")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    CELL_REF_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_REF_FIELD_NUMBER: _ClassVar[int]
    TRANSFORM_FIELD_NUMBER: _ClassVar[int]
    VIEW_REF_FIELD_NUMBER: _ClassVar[int]
    FIELDS_FIELD_NUMBER: _ClassVar[int]
    NET_ANCHOR_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    cell_ref: str
    library_ref: str
    transform: Transform
    view_ref: str
    fields: _containers.RepeatedCompositeFieldContainer[Field]
    net_anchor: str
    prov: Provenance
    def __init__(self, ref_des: _Optional[str] = ..., cell_ref: _Optional[str] = ..., library_ref: _Optional[str] = ..., transform: _Optional[_Union[Transform, _Mapping]] = ..., view_ref: _Optional[str] = ..., fields: _Optional[_Iterable[_Union[Field, _Mapping]]] = ..., net_anchor: _Optional[str] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Polyline(_message.Message):
    __slots__ = ("points",)
    POINTS_FIELD_NUMBER: _ClassVar[int]
    points: _containers.RepeatedCompositeFieldContainer[Point]
    def __init__(self, points: _Optional[_Iterable[_Union[Point, _Mapping]]] = ...) -> None: ...

class WireGeometry(_message.Message):
    __slots__ = ("net", "net_id", "polylines", "kind", "prov")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[WireGeometry.Kind]
        KIND_WIRE: _ClassVar[WireGeometry.Kind]
        KIND_BUS: _ClassVar[WireGeometry.Kind]
        KIND_BUS_ENTRY: _ClassVar[WireGeometry.Kind]
    KIND_UNSPECIFIED: WireGeometry.Kind
    KIND_WIRE: WireGeometry.Kind
    KIND_BUS: WireGeometry.Kind
    KIND_BUS_ENTRY: WireGeometry.Kind
    NET_FIELD_NUMBER: _ClassVar[int]
    NET_ID_FIELD_NUMBER: _ClassVar[int]
    POLYLINES_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    net: str
    net_id: str
    polylines: _containers.RepeatedCompositeFieldContainer[Polyline]
    kind: WireGeometry.Kind
    prov: Provenance
    def __init__(self, net: _Optional[str] = ..., net_id: _Optional[str] = ..., polylines: _Optional[_Iterable[_Union[Polyline, _Mapping]]] = ..., kind: _Optional[_Union[WireGeometry.Kind, str]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class Label(_message.Message):
    __slots__ = ("text", "origin", "height", "justify", "rotation_deg")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    text: str
    origin: Point
    height: int
    justify: str
    rotation_deg: int
    def __init__(self, text: _Optional[str] = ..., origin: _Optional[_Union[Point, _Mapping]] = ..., height: _Optional[int] = ..., justify: _Optional[str] = ..., rotation_deg: _Optional[int] = ...) -> None: ...

class SheetGeometry(_message.Message):
    __slots__ = ("id", "name", "parent_id", "size", "placements", "wires", "labels", "shapes", "title_block", "images", "suppress_worksheet", "prov")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARENT_ID_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    PLACEMENTS_FIELD_NUMBER: _ClassVar[int]
    WIRES_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    SHAPES_FIELD_NUMBER: _ClassVar[int]
    TITLE_BLOCK_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    SUPPRESS_WORKSHEET_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    parent_id: str
    size: BBox
    placements: _containers.RepeatedCompositeFieldContainer[SymbolPlacement]
    wires: _containers.RepeatedCompositeFieldContainer[WireGeometry]
    labels: _containers.RepeatedCompositeFieldContainer[Label]
    shapes: _containers.RepeatedCompositeFieldContainer[Shape]
    title_block: TitleBlock
    images: _containers.RepeatedCompositeFieldContainer[Image]
    suppress_worksheet: bool
    prov: Provenance
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., parent_id: _Optional[str] = ..., size: _Optional[_Union[BBox, _Mapping]] = ..., placements: _Optional[_Iterable[_Union[SymbolPlacement, _Mapping]]] = ..., wires: _Optional[_Iterable[_Union[WireGeometry, _Mapping]]] = ..., labels: _Optional[_Iterable[_Union[Label, _Mapping]]] = ..., shapes: _Optional[_Iterable[_Union[Shape, _Mapping]]] = ..., title_block: _Optional[_Union[TitleBlock, _Mapping]] = ..., images: _Optional[_Iterable[_Union[Image, _Mapping]]] = ..., suppress_worksheet: _Optional[bool] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class TitleBlock(_message.Message):
    __slots__ = ("title", "rev", "date", "company", "comments", "extra_fields")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    REV_FIELD_NUMBER: _ClassVar[int]
    DATE_FIELD_NUMBER: _ClassVar[int]
    COMPANY_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FIELDS_FIELD_NUMBER: _ClassVar[int]
    title: str
    rev: str
    date: str
    company: str
    comments: _containers.RepeatedScalarFieldContainer[str]
    extra_fields: _containers.RepeatedCompositeFieldContainer[KeyValue]
    def __init__(self, title: _Optional[str] = ..., rev: _Optional[str] = ..., date: _Optional[str] = ..., company: _Optional[str] = ..., comments: _Optional[_Iterable[str]] = ..., extra_fields: _Optional[_Iterable[_Union[KeyValue, _Mapping]]] = ...) -> None: ...

class KeyValue(_message.Message):
    __slots__ = ("key", "value")
    KEY_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    key: str
    value: str
    def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class SchematicGeometry(_message.Message):
    __slots__ = ("design_ref", "unit_nm", "symbols", "sheets", "undrawn", "prov")
    DESIGN_REF_FIELD_NUMBER: _ClassVar[int]
    UNIT_NM_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    SHEETS_FIELD_NUMBER: _ClassVar[int]
    UNDRAWN_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    design_ref: str
    unit_nm: int
    symbols: _containers.RepeatedCompositeFieldContainer[SymbolDef]
    sheets: _containers.RepeatedCompositeFieldContainer[SheetGeometry]
    undrawn: _containers.RepeatedCompositeFieldContainer[UndrawnPlacement]
    prov: Provenance
    def __init__(self, design_ref: _Optional[str] = ..., unit_nm: _Optional[int] = ..., symbols: _Optional[_Iterable[_Union[SymbolDef, _Mapping]]] = ..., sheets: _Optional[_Iterable[_Union[SheetGeometry, _Mapping]]] = ..., undrawn: _Optional[_Iterable[_Union[UndrawnPlacement, _Mapping]]] = ..., prov: _Optional[_Union[Provenance, _Mapping]] = ...) -> None: ...

class UndrawnPlacement(_message.Message):
    __slots__ = ("ref_des", "cell_ref", "library_ref", "sheet_id")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    CELL_REF_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_REF_FIELD_NUMBER: _ClassVar[int]
    SHEET_ID_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    cell_ref: str
    library_ref: str
    sheet_id: str
    def __init__(self, ref_des: _Optional[str] = ..., cell_ref: _Optional[str] = ..., library_ref: _Optional[str] = ..., sheet_id: _Optional[str] = ...) -> None: ...
