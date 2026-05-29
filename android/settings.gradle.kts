pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        google()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
        google()
        // JitPack til kotpass (keemobile publicerer via JitPack)
        maven { url = uri("https://jitpack.io") }
    }
}

rootProject.name = "deltasync"

include(":sync")
include(":app")
