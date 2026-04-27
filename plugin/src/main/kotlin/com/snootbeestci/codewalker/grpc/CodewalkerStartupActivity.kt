package com.snootbeestci.codewalker.grpc

import com.intellij.openapi.project.Project
import com.intellij.openapi.startup.ProjectActivity
import com.snootbeestci.codewalker.settings.CodewalkerSettings

class CodewalkerStartupActivity : ProjectActivity {
    override suspend fun execute(project: Project) {
        val address = CodewalkerSettings.getInstance().state.backendAddress
        CodewalkerClient.getInstance().connect(address)
    }
}
