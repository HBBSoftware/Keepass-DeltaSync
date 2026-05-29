// Top-level build-fil. Konkret kompilation sker i submoduler.

plugins {
    kotlin("jvm") version "2.0.20" apply false
    kotlin("android") version "2.0.20" apply false
    kotlin("plugin.serialization") version "2.0.20" apply false
    id("com.android.application") version "8.5.2" apply false
}
