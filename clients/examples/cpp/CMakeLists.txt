# The C++ client example for `make exercise-clients` (agni issue 1638). It adds the client library
# from this checkout as a subdirectory, as clients/cpp/README.md shows for any CMake project.
cmake_minimum_required(VERSION 3.21)
project(agni_cpp_example LANGUAGES CXX)

add_subdirectory("${CMAKE_CURRENT_SOURCE_DIR}/../../cpp" agni_client)

add_executable(agni_example main.cc)
target_link_libraries(agni_example PRIVATE agni::client)
