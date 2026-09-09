plugins {
    id("com.android.application")
}

android {
    namespace = "io.github.maas3n.mattmux"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.maas3n.mattmux"
        minSdk = 26
        targetSdk = 36
        versionCode = 100
        versionName = "1.2.0-chromeos-alpha1"

        // Keep purchases disabled until the Android-native remux engine is shipped.
        buildConfigField("boolean", "ENABLE_BILLING_PURCHASES", "false")
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    implementation("com.android.billingclient:billing:9.1.0")
}
