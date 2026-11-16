# agni's C++ client (agni issue 1638). It generates the messages and gRPC stubs from protos/agni/v1
# at build time and adds a thin layer over them (include/agni/client.h). Generated C++ only compiles
# against the protobuf runtime of the protoc that wrote it, so nothing generated is committed. Each
# build writes its own from the protos and the protoc it finds.
#
#   cmake -S clients/cpp -B clients/cpp/build && cmake --build clients/cpp/build
#   ctest --test-dir clients/cpp/build --output-on-failure
cmake_minimum_required(VERSION 3.21)
project(agni_client LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(AGNI_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../protos" CACHE PATH "the protos/ directory of an agni checkout")
option(AGNI_CLIENT_TESTS "build the client's tests, which need Go to build agni" ${PROJECT_IS_TOP_LEVEL})

# Protobuf and gRPC. A build from source or Homebrew installs CMake config files. Ubuntu's packages
# ship gRPC's but not protobuf's, so protobuf falls back to CMake's own FindProtobuf, and gRPC to
# pkg-config when its config files are missing too.
find_package(Protobuf CONFIG QUIET)
if(NOT Protobuf_FOUND)
  find_package(Protobuf REQUIRED)
endif()
find_package(gRPC CONFIG QUIET)
if(gRPC_FOUND)
  set(AGNI_GRPCPP gRPC::grpc++)
  set(AGNI_GRPC_PLUGIN $<TARGET_FILE:gRPC::grpc_cpp_plugin>)
else()
  find_package(PkgConfig REQUIRED)
  pkg_check_modules(GRPCPP REQUIRED IMPORTED_TARGET grpc++)
  set(AGNI_GRPCPP PkgConfig::GRPCPP)
  find_program(AGNI_GRPC_PLUGIN grpc_cpp_plugin REQUIRED)
endif()
if(TARGET protobuf::protoc)
  set(AGNI_PROTOC $<TARGET_FILE:protobuf::protoc>)
else()
  find_program(AGNI_PROTOC protoc REQUIRED)
endif()

# One protoc run writes the messages and the stubs for every proto under agni/v1.
file(GLOB_RECURSE AGNI_PROTOS CONFIGURE_DEPENDS RELATIVE "${AGNI_PROTO_DIR}" "${AGNI_PROTO_DIR}/agni/v1/*.proto")
if(NOT AGNI_PROTOS)
  message(FATAL_ERROR "no protos under ${AGNI_PROTO_DIR}/agni/v1; set AGNI_PROTO_DIR to an agni checkout's protos/")
endif()
set(AGNI_GEN_DIR "${CMAKE_CURRENT_BINARY_DIR}/gen")
set(AGNI_GEN_SOURCES)
set(AGNI_PROTO_PATHS)
foreach(p IN LISTS AGNI_PROTOS)
  string(REGEX REPLACE "\\.proto$" "" stem "${p}")
  list(APPEND AGNI_GEN_SOURCES
    "${AGNI_GEN_DIR}/${stem}.pb.cc" "${AGNI_GEN_DIR}/${stem}.pb.h"
    "${AGNI_GEN_DIR}/${stem}.grpc.pb.cc" "${AGNI_GEN_DIR}/${stem}.grpc.pb.h")
  list(APPEND AGNI_PROTO_PATHS "${AGNI_PROTO_DIR}/${p}")
endforeach()
add_custom_command(
  OUTPUT ${AGNI_GEN_SOURCES}
  COMMAND ${CMAKE_COMMAND} -E make_directory "${AGNI_GEN_DIR}"
  COMMAND ${AGNI_PROTOC}
    --proto_path=${AGNI_PROTO_DIR}
    --cpp_out=${AGNI_GEN_DIR}
    --grpc_out=${AGNI_GEN_DIR}
    --plugin=protoc-gen-grpc=${AGNI_GRPC_PLUGIN}
    ${AGNI_PROTOS}
  DEPENDS ${AGNI_PROTO_PATHS}
  WORKING_DIRECTORY "${AGNI_PROTO_DIR}"
  COMMENT "Generating C++ from ${AGNI_PROTO_DIR}/agni/v1"
  VERBATIM)

add_library(agni_client
  ${AGNI_GEN_SOURCES}
  src/client.cc)
add_library(agni::client ALIAS agni_client)
target_include_directories(agni_client PUBLIC
  "${CMAKE_CURRENT_SOURCE_DIR}/include"
  "${AGNI_GEN_DIR}")
target_link_libraries(agni_client PUBLIC protobuf::libprotobuf ${AGNI_GRPCPP})

if(AGNI_CLIENT_TESTS)
  enable_testing()
  add_subdirectory(test)
endif()
