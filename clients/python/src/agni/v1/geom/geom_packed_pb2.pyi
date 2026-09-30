from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class HighlightShape(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HIGHLIGHT_SHAPE_UNSPECIFIED: _ClassVar[HighlightShape]
    HIGHLIGHT_SHAPE_OUTLINE: _ClassVar[HighlightShape]
    HIGHLIGHT_SHAPE_BOUNDING_RECT: _ClassVar[HighlightShape]
    HIGHLIGHT_SHAPE_BOUNDING_CIRCLE: _ClassVar[HighlightShape]
    HIGHLIGHT_SHAPE_PATH: _ClassVar[HighlightShape]
HIGHLIGHT_SHAPE_UNSPECIFIED: HighlightShape
HIGHLIGHT_SHAPE_OUTLINE: HighlightShape
HIGHLIGHT_SHAPE_BOUNDING_RECT: HighlightShape
HIGHLIGHT_SHAPE_BOUNDING_CIRCLE: HighlightShape
HIGHLIGHT_SHAPE_PATH: HighlightShape

class PrimitiveKey(_message.Message):
    __slots__ = ("primitive", "ref_des", "net", "pin", "net_id", "bus_id")
    PRIMITIVE_FIELD_NUMBER: _ClassVar[int]
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    NET_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    NET_ID_FIELD_NUMBER: _ClassVar[int]
    BUS_ID_FIELD_NUMBER: _ClassVar[int]
    primitive: int
    ref_des: str
    net: str
    pin: str
    net_id: str
    bus_id: str
    def __init__(self, primitive: _Optional[int] = ..., ref_des: _Optional[str] = ..., net: _Optional[str] = ..., pin: _Optional[str] = ..., net_id: _Optional[str] = ..., bus_id: _Optional[str] = ...) -> None: ...

class PinRef(_message.Message):
    __slots__ = ("ref_des", "pin")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    pin: str
    def __init__(self, ref_des: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...

class HighlightSpec(_message.Message):
    __slots__ = ("components", "nets", "pins", "net_ids", "bus_ids", "color", "alpha", "shape", "stroke_scale")
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    PINS_FIELD_NUMBER: _ClassVar[int]
    NET_IDS_FIELD_NUMBER: _ClassVar[int]
    BUS_IDS_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    ALPHA_FIELD_NUMBER: _ClassVar[int]
    SHAPE_FIELD_NUMBER: _ClassVar[int]
    STROKE_SCALE_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedScalarFieldContainer[str]
    nets: _containers.RepeatedScalarFieldContainer[str]
    pins: _containers.RepeatedCompositeFieldContainer[PinRef]
    net_ids: _containers.RepeatedScalarFieldContainer[str]
    bus_ids: _containers.RepeatedScalarFieldContainer[str]
    color: str
    alpha: float
    shape: HighlightShape
    stroke_scale: float
    def __init__(self, components: _Optional[_Iterable[str]] = ..., nets: _Optional[_Iterable[str]] = ..., pins: _Optional[_Iterable[_Union[PinRef, _Mapping]]] = ..., net_ids: _Optional[_Iterable[str]] = ..., bus_ids: _Optional[_Iterable[str]] = ..., color: _Optional[str] = ..., alpha: _Optional[float] = ..., shape: _Optional[_Union[HighlightShape, str]] = ..., stroke_scale: _Optional[float] = ...) -> None: ...

class PackedHighlight(_message.Message):
    __slots__ = ("groups",)
    class Group(_message.Message):
        __slots__ = ("color", "alpha", "primitives", "shape")
        COLOR_FIELD_NUMBER: _ClassVar[int]
        ALPHA_FIELD_NUMBER: _ClassVar[int]
        PRIMITIVES_FIELD_NUMBER: _ClassVar[int]
        SHAPE_FIELD_NUMBER: _ClassVar[int]
        color: str
        alpha: float
        primitives: _containers.RepeatedScalarFieldContainer[int]
        shape: HighlightShape
        def __init__(self, color: _Optional[str] = ..., alpha: _Optional[float] = ..., primitives: _Optional[_Iterable[int]] = ..., shape: _Optional[_Union[HighlightShape, str]] = ...) -> None: ...
    GROUPS_FIELD_NUMBER: _ClassVar[int]
    groups: _containers.RepeatedCompositeFieldContainer[PackedHighlight.Group]
    def __init__(self, groups: _Optional[_Iterable[_Union[PackedHighlight.Group, _Mapping]]] = ...) -> None: ...

class PackedLabel(_message.Message):
    __slots__ = ("x", "y", "text", "height", "rotation_deg", "justify", "color", "max_width")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    MAX_WIDTH_FIELD_NUMBER: _ClassVar[int]
    x: int
    y: int
    text: str
    height: int
    rotation_deg: int
    justify: str
    color: str
    max_width: int
    def __init__(self, x: _Optional[int] = ..., y: _Optional[int] = ..., text: _Optional[str] = ..., height: _Optional[int] = ..., rotation_deg: _Optional[int] = ..., justify: _Optional[str] = ..., color: _Optional[str] = ..., max_width: _Optional[int] = ...) -> None: ...

class PackedImage(_message.Message):
    __slots__ = ("x", "y", "w", "h", "mime", "data", "rotation_deg", "mirror")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    W_FIELD_NUMBER: _ClassVar[int]
    H_FIELD_NUMBER: _ClassVar[int]
    MIME_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    MIRROR_FIELD_NUMBER: _ClassVar[int]
    x: int
    y: int
    w: int
    h: int
    mime: str
    data: bytes
    rotation_deg: int
    mirror: bool
    def __init__(self, x: _Optional[int] = ..., y: _Optional[int] = ..., w: _Optional[int] = ..., h: _Optional[int] = ..., mime: _Optional[str] = ..., data: _Optional[bytes] = ..., rotation_deg: _Optional[int] = ..., mirror: _Optional[bool] = ...) -> None: ...

class PackedSheet(_message.Message):
    __slots__ = ("sheet_id", "layout_version", "origin_x", "origin_y", "vertices", "primitives", "keys", "labels", "font_family", "group_colors", "background_color", "images")
    SHEET_ID_FIELD_NUMBER: _ClassVar[int]
    LAYOUT_VERSION_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_X_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_Y_FIELD_NUMBER: _ClassVar[int]
    VERTICES_FIELD_NUMBER: _ClassVar[int]
    PRIMITIVES_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    FONT_FAMILY_FIELD_NUMBER: _ClassVar[int]
    GROUP_COLORS_FIELD_NUMBER: _ClassVar[int]
    BACKGROUND_COLOR_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    sheet_id: str
    layout_version: int
    origin_x: int
    origin_y: int
    vertices: bytes
    primitives: bytes
    keys: _containers.RepeatedCompositeFieldContainer[PrimitiveKey]
    labels: _containers.RepeatedCompositeFieldContainer[PackedLabel]
    font_family: str
    group_colors: _containers.RepeatedScalarFieldContainer[str]
    background_color: str
    images: _containers.RepeatedCompositeFieldContainer[PackedImage]
    def __init__(self, sheet_id: _Optional[str] = ..., layout_version: _Optional[int] = ..., origin_x: _Optional[int] = ..., origin_y: _Optional[int] = ..., vertices: _Optional[bytes] = ..., primitives: _Optional[bytes] = ..., keys: _Optional[_Iterable[_Union[PrimitiveKey, _Mapping]]] = ..., labels: _Optional[_Iterable[_Union[PackedLabel, _Mapping]]] = ..., font_family: _Optional[str] = ..., group_colors: _Optional[_Iterable[str]] = ..., background_color: _Optional[str] = ..., images: _Optional[_Iterable[_Union[PackedImage, _Mapping]]] = ...) -> None: ...
