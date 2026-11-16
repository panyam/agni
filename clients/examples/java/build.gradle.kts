// The Java example for `make exercise-clients` (agni issue 1632). It builds inside clients/java's
// Gradle build, as the project :example, so it compiles against the client's source. The run script
// beside this file builds it with installDist and runs the launcher that writes.
plugins {
    application
}

dependencies {
    implementation(project(":"))
}

application {
    mainClass = "io.github.panyam.agni.example.Main"
    applicationName = "agni-java-example"
}

tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
    options.release = 21
}
