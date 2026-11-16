# Two tests. client_test holds the thin layer to the contract against a stand-in ServerService in the
# same process. wire_fixtures starts agni serve and replays clients/wirefixtures/fixtures.json through
# the client, as cmd/agni's TestWireFixtures does over Connect and grpc-go.
get_filename_component(AGNI_REPO "${CMAKE_CURRENT_SOURCE_DIR}/../../.." ABSOLUTE)
set(AGNI_BINARY "" CACHE FILEPATH "the agni binary the replayer serves; empty builds one with go")

# The fixture file's own shape, read with protobuf's JSON parser so the tests need no JSON library.
add_custom_command(
  OUTPUT "${CMAKE_CURRENT_BINARY_DIR}/wirefixtures.pb.cc" "${CMAKE_CURRENT_BINARY_DIR}/wirefixtures.pb.h"
  COMMAND ${AGNI_PROTOC} --proto_path=${CMAKE_CURRENT_SOURCE_DIR} --cpp_out=${CMAKE_CURRENT_BINARY_DIR} wirefixtures.proto
  DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/wirefixtures.proto"
  VERBATIM)

add_executable(wirefixtures_test wirefixtures_test.cc "${CMAKE_CURRENT_BINARY_DIR}/wirefixtures.pb.cc")
target_include_directories(wirefixtures_test PRIVATE "${CMAKE_CURRENT_BINARY_DIR}")
target_link_libraries(wirefixtures_test PRIVATE agni::client)

add_executable(client_test client_test.cc)
target_link_libraries(client_test PRIVATE agni::client)

add_test(NAME client COMMAND client_test)

if(AGNI_BINARY)
  set(agni_bin "${AGNI_BINARY}")
else()
  find_program(AGNI_GO go REQUIRED)
  set(agni_bin "${CMAKE_CURRENT_BINARY_DIR}/agni")
  add_test(NAME build_agni COMMAND "${AGNI_GO}" build -o "${agni_bin}" ./cmd/agni WORKING_DIRECTORY "${AGNI_REPO}")
  set_tests_properties(build_agni PROPERTIES FIXTURES_SETUP agni TIMEOUT 900)
endif()
add_test(NAME wire_fixtures COMMAND wirefixtures_test
  --agni "${agni_bin}"
  --mount "probe=${AGNI_REPO}/examples/cross-probe-project"
  --fixtures "${AGNI_REPO}/clients/wirefixtures/fixtures.json")
set_tests_properties(wire_fixtures PROPERTIES FIXTURES_REQUIRED agni TIMEOUT 300)
