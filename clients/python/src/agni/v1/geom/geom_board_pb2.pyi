from agni.v1.geom import geom_pb2 as _geom_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class BoardGeometry(_message.Message):
    __slots__ = ("design_ref", "unit_nm", "layers", "outline", "placements", "nets", "zones", "texts", "graphics", "prov")
    DESIGN_REF_FIELD_NUMBER: _ClassVar[int]
    UNIT_NM_FIELD_NUMBER: _ClassVar[int]
    LAYERS_FIELD_NUMBER: _ClassVar[int]
    OUTLINE_FIELD_NUMBER: _ClassVar[int]
    PLACEMENTS_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    ZONES_FIELD_NUMBER: _ClassVar[int]
    TEXTS_FIELD_NUMBER: _ClassVar[int]
    GRAPHICS_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    design_ref: str
    unit_nm: int
    layers: _containers.RepeatedCompositeFieldContainer[BoardLayer]
    outline: BoardOutline
    placements: _containers.RepeatedCompositeFieldContainer[ComponentPlacement]
    nets: _containers.RepeatedCompositeFieldContainer[NetCopper]
    zones: _containers.RepeatedCompositeFieldContainer[Zone]
    texts: _containers.RepeatedCompositeFieldContainer[BoardText]
    graphics: _containers.RepeatedCompositeFieldContainer[BoardGraphic]
    prov: _geom_pb2.Provenance
    def __init__(self, design_ref: _Optional[str] = ..., unit_nm: _Optional[int] = ..., layers: _Optional[_Iterable[_Union[BoardLayer, _Mapping]]] = ..., outline: _Optional[_Union[BoardOutline, _Mapping]] = ..., placements: _Optional[_Iterable[_Union[ComponentPlacement, _Mapping]]] = ..., nets: _Optional[_Iterable[_Union[NetCopper, _Mapping]]] = ..., zones: _Optional[_Iterable[_Union[Zone, _Mapping]]] = ..., texts: _Optional[_Iterable[_Union[BoardText, _Mapping]]] = ..., graphics: _Optional[_Iterable[_Union[BoardGraphic, _Mapping]]] = ..., prov: _Optional[_Union[_geom_pb2.Provenance, _Mapping]] = ...) -> None: ...

class BoardText(_message.Message):
    __slots__ = ("text", "at", "rotation_deg", "height", "layer", "mirror", "justify", "ref_des", "kind")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    LAYER_FIELD_NUMBER: _ClassVar[int]
    MIRROR_FIELD_NUMBER: _ClassVar[int]
    JUSTIFY_FIELD_NUMBER: _ClassVar[int]
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    text: str
    at: _geom_pb2.Point
    rotation_deg: float
    height: int
    layer: str
    mirror: bool
    justify: str
    ref_des: str
    kind: str
    def __init__(self, text: _Optional[str] = ..., at: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., rotation_deg: _Optional[float] = ..., height: _Optional[int] = ..., layer: _Optional[str] = ..., mirror: _Optional[bool] = ..., justify: _Optional[str] = ..., ref_des: _Optional[str] = ..., kind: _Optional[str] = ...) -> None: ...

class BoardGraphic(_message.Message):
    __slots__ = ("shape", "layer", "ref_des", "width")
    SHAPE_FIELD_NUMBER: _ClassVar[int]
    LAYER_FIELD_NUMBER: _ClassVar[int]
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    shape: _geom_pb2.Shape
    layer: str
    ref_des: str
    width: int
    def __init__(self, shape: _Optional[_Union[_geom_pb2.Shape, _Mapping]] = ..., layer: _Optional[str] = ..., ref_des: _Optional[str] = ..., width: _Optional[int] = ...) -> None: ...

class BoardLayer(_message.Message):
    __slots__ = ("number", "name", "kind")
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    number: int
    name: str
    kind: str
    def __init__(self, number: _Optional[int] = ..., name: _Optional[str] = ..., kind: _Optional[str] = ...) -> None: ...

class BoardOutline(_message.Message):
    __slots__ = ("paths",)
    PATHS_FIELD_NUMBER: _ClassVar[int]
    paths: _containers.RepeatedCompositeFieldContainer[_geom_pb2.Polyline]
    def __init__(self, paths: _Optional[_Iterable[_Union[_geom_pb2.Polyline, _Mapping]]] = ...) -> None: ...

class ComponentPlacement(_message.Message):
    __slots__ = ("ref_des", "at", "rotation_deg", "layer", "pads", "mirror", "prov")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    LAYER_FIELD_NUMBER: _ClassVar[int]
    PADS_FIELD_NUMBER: _ClassVar[int]
    MIRROR_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    at: _geom_pb2.Point
    rotation_deg: float
    layer: str
    pads: _containers.RepeatedCompositeFieldContainer[Pad]
    mirror: bool
    prov: _geom_pb2.Provenance
    def __init__(self, ref_des: _Optional[str] = ..., at: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., rotation_deg: _Optional[float] = ..., layer: _Optional[str] = ..., pads: _Optional[_Iterable[_Union[Pad, _Mapping]]] = ..., mirror: _Optional[bool] = ..., prov: _Optional[_Union[_geom_pb2.Provenance, _Mapping]] = ...) -> None: ...

class Pad(_message.Message):
    __slots__ = ("number", "at", "rotation_deg", "size", "shape", "layers", "drill", "net")
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    ROTATION_DEG_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    SHAPE_FIELD_NUMBER: _ClassVar[int]
    LAYERS_FIELD_NUMBER: _ClassVar[int]
    DRILL_FIELD_NUMBER: _ClassVar[int]
    NET_FIELD_NUMBER: _ClassVar[int]
    number: str
    at: _geom_pb2.Point
    rotation_deg: float
    size: _geom_pb2.Point
    shape: str
    layers: _containers.RepeatedScalarFieldContainer[str]
    drill: int
    net: str
    def __init__(self, number: _Optional[str] = ..., at: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., rotation_deg: _Optional[float] = ..., size: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., shape: _Optional[str] = ..., layers: _Optional[_Iterable[str]] = ..., drill: _Optional[int] = ..., net: _Optional[str] = ...) -> None: ...

class NetCopper(_message.Message):
    __slots__ = ("net", "segments", "vias")
    NET_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    VIAS_FIELD_NUMBER: _ClassVar[int]
    net: str
    segments: _containers.RepeatedCompositeFieldContainer[TrackSegment]
    vias: _containers.RepeatedCompositeFieldContainer[Via]
    def __init__(self, net: _Optional[str] = ..., segments: _Optional[_Iterable[_Union[TrackSegment, _Mapping]]] = ..., vias: _Optional[_Iterable[_Union[Via, _Mapping]]] = ...) -> None: ...

class TrackSegment(_message.Message):
    __slots__ = ("a", "b", "width", "layer")
    A_FIELD_NUMBER: _ClassVar[int]
    B_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    LAYER_FIELD_NUMBER: _ClassVar[int]
    a: _geom_pb2.Point
    b: _geom_pb2.Point
    width: int
    layer: str
    def __init__(self, a: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., b: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., width: _Optional[int] = ..., layer: _Optional[str] = ...) -> None: ...

class Via(_message.Message):
    __slots__ = ("at", "size", "drill", "layer_from", "layer_to")
    AT_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    DRILL_FIELD_NUMBER: _ClassVar[int]
    LAYER_FROM_FIELD_NUMBER: _ClassVar[int]
    LAYER_TO_FIELD_NUMBER: _ClassVar[int]
    at: _geom_pb2.Point
    size: int
    drill: int
    layer_from: str
    layer_to: str
    def __init__(self, at: _Optional[_Union[_geom_pb2.Point, _Mapping]] = ..., size: _Optional[int] = ..., drill: _Optional[int] = ..., layer_from: _Optional[str] = ..., layer_to: _Optional[str] = ...) -> None: ...

class Zone(_message.Message):
    __slots__ = ("net", "layer", "outline")
    NET_FIELD_NUMBER: _ClassVar[int]
    LAYER_FIELD_NUMBER: _ClassVar[int]
    OUTLINE_FIELD_NUMBER: _ClassVar[int]
    net: str
    layer: str
    outline: _geom_pb2.Polyline
    def __init__(self, net: _Optional[str] = ..., layer: _Optional[str] = ..., outline: _Optional[_Union[_geom_pb2.Polyline, _Mapping]] = ...) -> None: ...
