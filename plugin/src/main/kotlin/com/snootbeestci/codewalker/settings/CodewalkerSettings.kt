package com.snootbeestci.codewalker.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage

@Service
@State(name = "CodewalkerSettings", storages = [Storage("codewalker.xml")])
class CodewalkerSettings : PersistentStateComponent<CodewalkerSettings.State> {

    data class State(
        var backendAddress: String = "localhost:50051"
    )

    private var myState = State()

    override fun getState(): State = myState

    override fun loadState(state: State) {
        myState = state
    }

    companion object {
        fun getInstance(): CodewalkerSettings =
            ApplicationManager.getApplication().getService(CodewalkerSettings::class.java)
    }
}
