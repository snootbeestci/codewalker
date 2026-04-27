package com.snootbeestci.codewalker.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.options.Configurable
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import com.snootbeestci.codewalker.grpc.CodewalkerClient
import kotlinx.coroutines.launch
import javax.swing.JComponent
import javax.swing.JPanel

class CodewalkerSettingsConfigurable : Configurable {

    private var addressField: JBTextField? = null

    override fun getDisplayName(): String = "Codewalker"

    override fun createComponent(): JComponent {
        addressField = JBTextField()
        return FormBuilder.createFormBuilder()
            .addLabeledComponent("Backend address:", addressField!!)
            .addComponentFillVertically(JPanel(), 0)
            .panel
    }

    override fun isModified(): Boolean {
        val state = CodewalkerSettings.getInstance().state
        return addressField?.text != state.backendAddress
    }

    override fun apply() {
        val state = CodewalkerSettings.getInstance().state
        state.backendAddress = addressField?.text ?: state.backendAddress
        ApplicationManager.getApplication().coroutineScope.launch {
            CodewalkerClient.getInstance().connect(
                CodewalkerSettings.getInstance().state.backendAddress
            )
        }
    }

    override fun reset() {
        val state = CodewalkerSettings.getInstance().state
        addressField?.text = state.backendAddress
    }

    override fun disposeUIResources() {
        addressField = null
    }
}
