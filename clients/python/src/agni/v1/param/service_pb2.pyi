from agni.v1.param import param_pb2 as _param_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class BatchGetPartSpecsRequest(_message.Message):
    __slots__ = ("mpns",)
    MPNS_FIELD_NUMBER: _ClassVar[int]
    mpns: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, mpns: _Optional[_Iterable[str]] = ...) -> None: ...

class BatchGetPartSpecsResponse(_message.Message):
    __slots__ = ("specs", "generation")
    SPECS_FIELD_NUMBER: _ClassVar[int]
    GENERATION_FIELD_NUMBER: _ClassVar[int]
    specs: _containers.RepeatedCompositeFieldContainer[_param_pb2.PartSpec]
    generation: int
    def __init__(self, specs: _Optional[_Iterable[_Union[_param_pb2.PartSpec, _Mapping]]] = ..., generation: _Optional[int] = ...) -> None: ...

class GetGenerationRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetGenerationResponse(_message.Message):
    __slots__ = ("generation",)
    GENERATION_FIELD_NUMBER: _ClassVar[int]
    generation: int
    def __init__(self, generation: _Optional[int] = ...) -> None: ...
