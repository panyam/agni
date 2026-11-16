// The Java client (agni issue 1632) and the exercise example that calls a server through it. The
// example lives under clients/examples/java beside the other languages' examples, and builds here so
// it compiles against this client's source rather than a published jar.
rootProject.name = "agni-client"

include(":example")
project(":example").projectDir = file("../examples/java")

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}
