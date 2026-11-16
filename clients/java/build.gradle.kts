// agni's Java client (agni issue 1632): the messages and gRPC stubs generated from protos/agni/v1,
// and a thin layer that connects, checks the server and reads error details. Nothing is published
// yet; a later ticket publishes to Maven Central.
plugins {
    `java-library`
}

group = "io.github.panyam.agni"
version = "0.0.0-SNAPSHOT"

// protobuf-java refuses generated code newer than itself. The protocolbuffers/java v36.2 plugin in
// buf.gen.java.yaml writes code for 4.36.2, and grpc/java v1.85.1 writes stubs for grpc-java 1.85.1.
val protobufVersion = "4.36.2"
val grpcVersion = "1.85.1"

// `make proto-java` writes the generated code into src/generated/java, and `make proto-check` holds
// it fresh. It compiles with the hand-written code as one source set.
sourceSets {
    main {
        java {
            srcDir("src/generated/java")
        }
    }
}

dependencies {
    api("com.google.protobuf:protobuf-java:$protobufVersion")
    api("io.grpc:grpc-protobuf:$grpcVersion")
    api("io.grpc:grpc-stub:$grpcVersion")
    implementation("io.grpc:grpc-netty-shaded:$grpcVersion")

    testImplementation("com.google.protobuf:protobuf-java-util:$protobufVersion")
    testImplementation("com.google.code.gson:gson:2.14.0")
    testImplementation(platform("org.junit:junit-bom:6.1.3"))
    testImplementation("org.junit.jupiter:junit-jupiter")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
    options.release = 21
}

// The replayer starts `agni serve`. AGNI_BIN names a built agni to use; without it the test builds
// one from the repo root with `go build`.
tasks.test {
    useJUnitPlatform()
    testLogging {
        events("passed", "failed", "skipped")
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
    }
    systemProperty("agni.repoRoot", rootDir.resolve("../..").canonicalPath)
    // The test replays against a server built from the repo's Go. Gradle does not track that source,
    // so the test runs every time rather than reading as up to date.
    outputs.upToDateWhen { false }
}
