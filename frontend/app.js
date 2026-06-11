const API_BASE = "http://localhost"; // Docker endpoint route

// Languages Templates Boilerplates
const TEMPLATES = {
  python: `# Online Python Compiler (IntelliSense Enabled)
import sys

def main():
    # Read inputs if required
    # input_data = sys.stdin.read().split()
    print("Hello from Python 3.10!")

if __name__ == '__main__':
    main()`,

  cpp: `// Online C++ Compiler (IntelliSense/Suggestions Enabled)
#include <iostream>
#include <vector>
#include <string>
#include <algorithm>

using namespace std;

int main() {
    // Optimizing input/output operations
    ios_base::sync_with_stdio(false);
    cin.tie(NULL);
    
    cout << "Hello from C++13!" << endl;
    
    return 0;
}`,
};

let editor = null;
let isProgrammaticChange = false;

function setEditorValue(val) {
  if (!editor) return;
  isProgrammaticChange = true;
  editor.setValue(val);
  isProgrammaticChange = false;
}

// Initialize Theme preference before rendering icons to ensure correct load
const savedTheme = localStorage.getItem("ide-theme") || "dark";
const isLight = savedTheme === "light";
if (isLight) {
  document.body.classList.add("light-mode");
  document.getElementById("themeIcon").setAttribute("data-lucide", "moon");
} else {
  document.getElementById("themeIcon").setAttribute("data-lucide", "sun");
}

// Synchronize initial language tab name and icon dynamically
const initialLanguageForLabel = document.getElementById("language").value;
document.getElementById("extLabel").innerText =
  initialLanguageForLabel === "python" ? "py" : "cpp";
document
  .getElementById("fileTabIcon")
  .setAttribute(
    "data-lucide",
    initialLanguageForLabel === "python" ? "file-code" : "file-code-2",
  );

// Initialize Lucide Icons
lucide.createIcons();

// Cache controls
function getSavedCode(lang) {
  const saved = localStorage.getItem(`code_${lang}`);
  return saved !== null ? saved : TEMPLATES[lang];
}

function saveCode(lang, code) {
  localStorage.setItem(`code_${lang}`, code);
}

// Configure Monaco Editor loading using AMD Loader
require.config({
  paths: {
    vs: "https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.39.0/min/vs",
  },
});
require(["vs/editor/editor.main"], function () {
  // High-contrast deep slate SaaS layout theme
  monaco.editor.defineTheme("saas-dark", {
    base: "vs-dark",
    inherit: true,
    rules: [
      { token: "comment", foreground: "52525b", fontStyle: "italic" },
      { token: "keyword", foreground: "a5b4fc" },
      { token: "string", foreground: "a7f3d0" },
      { token: "number", foreground: "fde047" },
      { token: "regexp", foreground: "fda4af" },
      { token: "type", foreground: "93c5fd" },
      { token: "class", foreground: "93c5fd" },
      { token: "function", foreground: "38bdf8" },
      { token: "variable", foreground: "fafafa" },
    ],
    colors: {
      "editor.background": "#0f0f13",
      "editor.foreground": "#f4f4f5",
      "editor.lineHighlightBackground": "#15151b",
      "editorLineNumber.foreground": "#454553",
      "editorLineNumber.activeForeground": "#a1a1aa",
      "editor.selectionBackground": "#3e3e4f",
      "editor.inactiveSelectionBackground": "#282835",
      "editorWidget.background": "#141418",
      "editorWidget.border": "#1e1e24",
      "input.background": "#09090b",
      "input.foreground": "#f4f4f5",
      "input.border": "#1e1e24",
      "dropdown.background": "#0f0f13",
      "dropdown.foreground": "#f4f4f5",
      "dropdown.border": "#1e1e24",
    },
  });

  // Clean light layout theme
  monaco.editor.defineTheme("saas-light", {
    base: "vs",
    inherit: true,
    rules: [
      { token: "comment", foreground: "71717a", fontStyle: "italic" },
      { token: "keyword", foreground: "4f46e5" },
      { token: "string", foreground: "059669" },
      { token: "number", foreground: "d97706" },
      { token: "regexp", foreground: "e11d48" },
      { token: "type", foreground: "2563eb" },
      { token: "class", foreground: "2563eb" },
      { token: "function", foreground: "0284c7" },
      { token: "variable", foreground: "09090b" },
    ],
    colors: {
      "editor.background": "#ffffff",
      "editor.foreground": "#09090b",
      "editor.lineHighlightBackground": "#f4f4f5",
      "editorLineNumber.foreground": "#a1a1aa",
      "editorLineNumber.activeForeground": "#71717a",
      "editor.selectionBackground": "#cbd5e1",
      "editor.inactiveSelectionBackground": "#e2e8f0",
      "editorWidget.background": "#f4f4f5",
      "editorWidget.border": "#e4e4e7",
      "input.background": "#ffffff",
      "input.foreground": "#09090b",
      "input.border": "#e4e4e7",
      "dropdown.background": "#ffffff",
      "dropdown.foreground": "#09090b",
      "dropdown.border": "#e4e4e7",
    },
  });

  const initialLanguage = document.getElementById("language").value;
  const savedCode = getSavedCode(initialLanguage);
  populateTemplateSelector();

  // Load configs
  const fontSize = parseInt(
    localStorage.getItem("ide-font-size") || "14",
  );
  const tabSize = parseInt(localStorage.getItem("ide-tabsize") || "4");
  const wordWrap =
    localStorage.getItem("ide-wordwrap") === "true" ? "on" : "off";
  const minimap = localStorage.getItem("ide-minimap") !== "false";

  // Create Monaco instance
  editor = monaco.editor.create(
    document.getElementById("codeEditorContainer"),
    {
      value: savedCode,
      language: initialLanguage === "cpp" ? "cpp" : "python",
      theme: isLight ? "saas-light" : "saas-dark",
      fontSize: fontSize,
      tabSize: tabSize,
      wordWrap: wordWrap,
      minimap: { enabled: minimap },
      automaticLayout: true,
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      fontLigatures: true,
      cursorBlinking: "smooth",
      cursorSmoothCaretAnimation: "on",
      smoothScrolling: true,
      padding: { top: 12, bottom: 12 },
    },
  );

  // Save on edits
  editor.onDidChangeModelContent(() => {
    const code = editor.getValue();
    const lang = document.getElementById("language").value;
    saveCode(lang, code);

    // Reset selector to placeholder only if change was made by typing
    if (!isProgrammaticChange) {
      const selector = document.getElementById("templateSelector");
      if (selector && selector.value !== "") {
        selector.value = ""; // Resets to "Templates" placeholder
      }
    }
  });

  // Set control elements states
  document.getElementById("settingFontSize").value = fontSize.toString();
  document.getElementById("settingTabSize").value = tabSize.toString();
  document.getElementById("settingWordWrap").checked = wordWrap === "on";
  document.getElementById("settingMinimap").checked = minimap;
});

// Toggle Light/Dark mode
function toggleTheme() {
  const body = document.body;
  const isL = body.classList.toggle("light-mode");
  localStorage.setItem("ide-theme", isL ? "light" : "dark");

  const icon = document.getElementById("themeIcon");
  if (isL) {
    icon.setAttribute("data-lucide", "moon");
  } else {
    icon.setAttribute("data-lucide", "sun");
  }
  lucide.createIcons();

  if (editor) {
    monaco.editor.setTheme(isL ? "saas-light" : "saas-dark");
  }

  showToast(`Switched to ${isL ? "Light" : "Dark"} theme`, "info");
}

// Handle language updates
document
  .getElementById("language")
  .addEventListener("change", function (e) {
    const newLang = e.target.value;
    const prevLang = newLang === "python" ? "cpp" : "python";

    if (editor) {
      saveCode(prevLang, editor.getValue());
      const code = getSavedCode(newLang);
      setEditorValue(code);

      const model = editor.getModel();
      monaco.editor.setModelLanguage(
        model,
        newLang === "cpp" ? "cpp" : "python",
      );
    }

    document.getElementById("extLabel").innerText =
      newLang === "python" ? "py" : "cpp";
    document
      .getElementById("fileTabIcon")
      .setAttribute(
        "data-lucide",
        newLang === "python" ? "file-code" : "file-code-2",
      );
    lucide.createIcons();
    populateTemplateSelector();
  });

// Sidebar settings toggler
function toggleSettings() {
  const sidebar = document.getElementById("settingsSidebar");
  const overlay = document.querySelector(".sidebar-overlay");
  sidebar.classList.toggle("open");
  overlay.classList.toggle("open");
}

// Settings controllers
document
  .getElementById("settingFontSize")
  .addEventListener("change", function (e) {
    const size = parseInt(e.target.value);
    localStorage.setItem("ide-font-size", size);
    if (editor) {
      editor.updateOptions({ fontSize: size });
    }
  });

document
  .getElementById("settingTabSize")
  .addEventListener("change", function (e) {
    const size = parseInt(e.target.value);
    localStorage.setItem("ide-tabsize", size);
    if (editor) {
      editor.getModel().updateOptions({ tabSize: size });
    }
  });

document
  .getElementById("settingWordWrap")
  .addEventListener("change", function (e) {
    const enabled = e.target.checked;
    localStorage.setItem("ide-wordwrap", enabled);
    if (editor) {
      editor.updateOptions({ wordWrap: enabled ? "on" : "off" });
    }
  });

document
  .getElementById("settingMinimap")
  .addEventListener("change", function (e) {
    const enabled = e.target.checked;
    localStorage.setItem("ide-minimap", enabled);
    if (editor) {
      editor.updateOptions({ minimap: { enabled: enabled } });
    }
  });

// Drag panel resizing controls
const workspace = document.querySelector(".workspace");
const editorPane = document.querySelector(".editor-pane");
const ioPane = document.querySelector(".io-pane");
const stdinSection = document.querySelector(".stdin-section");
const resizerV = document.querySelector(".resizer-v");
const resizerH = document.querySelector(".resizer-h");

let isDraggingV = false;
let isDraggingH = false;

resizerV.addEventListener("mousedown", (e) => {
  isDraggingV = true;
  resizerV.classList.add("dragging");
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
});

resizerH.addEventListener("mousedown", (e) => {
  isDraggingH = true;
  resizerH.classList.add("dragging");
  document.body.style.cursor = "row-resize";
  document.body.style.userSelect = "none";
});

document.addEventListener("mousemove", (e) => {
  if (isDraggingV) {
    const workspaceRect = workspace.getBoundingClientRect();
    const offsetLeft = e.clientX - workspaceRect.left;

    if (offsetLeft > 300 && offsetLeft < workspaceRect.width - 300) {
      const percentage = (offsetLeft / workspaceRect.width) * 100;
      editorPane.style.width = `${percentage}%`;
      ioPane.style.width = `${100 - percentage}%`;
      if (editor) editor.layout();
    }
  }

  if (isDraggingH) {
    const ioRect = ioPane.getBoundingClientRect();
    const offsetTop = e.clientY - ioRect.top;

    if (offsetTop > 80 && offsetTop < ioRect.height - 80) {
      const percentage = (offsetTop / ioRect.height) * 100;
      stdinSection.style.height = `${percentage}%`;
    }
  }
});

document.addEventListener("mouseup", () => {
  if (isDraggingV) {
    isDraggingV = false;
    resizerV.classList.remove("dragging");
  }
  if (isDraggingH) {
    isDraggingH = false;
    resizerH.classList.remove("dragging");
  }
  document.body.style.cursor = "default";
  document.body.style.userSelect = "auto";
});

resizerV.addEventListener("dblclick", () => {
  editorPane.style.width = "65%";
  ioPane.style.width = "35%";
  if (editor) editor.layout();
});

resizerH.addEventListener("dblclick", () => {
  stdinSection.style.height = "40%";
});

// Actions details
function copyCode() {
  if (!editor) return;
  navigator.clipboard
    .writeText(editor.getValue())
    .then(() => showToast("Copied code to clipboard", "success"))
    .catch(() => showToast("Failed to copy code", "error"));
}

function downloadCode() {
  if (!editor) return;
  const code = editor.getValue();
  const lang = document.getElementById("language").value;
  const ext = lang === "python" ? "py" : "cpp";
  const blob = new Blob([code], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `main.${ext}`;
  a.click();
  URL.revokeObjectURL(url);
  showToast("Downloaded file successfully", "success");
}

function triggerUpload() {
  document.getElementById("fileInput").click();
}

function handleFileUpload(e) {
  const file = e.target.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = function (evt) {
    if (editor) {
      setEditorValue(evt.target.result);
      showToast(`Imported ${file.name}`, "info");
    }
  };
  reader.readAsText(file);
  e.target.value = "";
}

function resetCode() {
  const lang = document.getElementById("language").value;
  if (
    confirm(
      "Are you sure you want to reset the editor? Your active draft will be replaced.",
    )
  ) {
    if (editor) {
      setEditorValue(TEMPLATES[lang]);
      saveCode(lang, TEMPLATES[lang]);
      showToast("Reset code draft to boilerplate", "info");
    }
  }
}

function toggleFullscreen() {
  const workspaceEl = document.querySelector(".workspace");
  const fsIcon = document.getElementById("fsIcon");

  if (!document.fullscreenElement) {
    workspaceEl.requestFullscreen().then(() => {
      fsIcon.setAttribute("data-lucide", "minimize-2");
      lucide.createIcons();
      showToast("Entered fullscreen mode", "info");
    });
  } else {
    document.exitFullscreen().then(() => {
      fsIcon.setAttribute("data-lucide", "maximize-2");
      lucide.createIcons();
    });
  }
}

document.addEventListener("fullscreenchange", () => {
  if (editor) {
    setTimeout(() => editor.layout(), 100);
  }
});

function copyStdin() {
  const stdinVal = document.getElementById("stdinEditor").value;
  if (!stdinVal) {
    showToast("Standard input is empty", "warning");
    return;
  }
  navigator.clipboard
    .writeText(stdinVal)
    .then(() => showToast("Copied standard input to clipboard", "success"))
    .catch(() => showToast("Failed to copy standard input", "error"));
}

function clearStdin() {
  document.getElementById("stdinEditor").value = "";
  showToast("Standard input cleared", "info");
}

// Clear helper functions
function clearOutput() {
  const out = document.getElementById("outputContent");
  out.innerHTML = `
    <div class="placeholder-text" id="outputPlaceholder">
      <i data-lucide="terminal" style="width: 20px; height: 20px"></i>
      <span>Output streams will be rendered here.</span>
    </div>
  `;
  lucide.createIcons();
  document.getElementById("statusBadge").style.display = "none";
  document.getElementById("metaInfo").style.display = "none";
  document.getElementById("copyOutputBtn").style.display = "none";
}

// Shortcut listener
document.addEventListener("keydown", function (e) {
  if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
    e.preventDefault();
    submitCode();
  }
});

// Toast Notification helper
function showToast(message, type = "success") {
  const container = document.getElementById("toastContainer");
  const toast = document.createElement("div");
  toast.className = `toast ${type}`;

  toast.innerHTML = `
    <span>${message}</span>
  `;
  container.appendChild(toast);

  setTimeout(() => toast.classList.add("show"), 10);

  setTimeout(() => {
    toast.classList.remove("show");
    setTimeout(() => toast.remove(), 300);
  }, 3000);
}

// Submit run
async function submitCode() {
  if (!editor) return;
  const code = editor.getValue();
  const input = document.getElementById("stdinEditor").value;
  const language = document.getElementById("language").value;
  const runBtn = document.getElementById("runBtn");
  const outContent = document.getElementById("outputContent");

  if (!code.trim()) {
    showToast("Please enter code before running", "error");
    return;
  }

  runBtn.disabled = true;
  const runText = document.getElementById("runText");
  const runIcon = document.getElementById("runIcon");
  runText.innerText = "Running...";
  runIcon.setAttribute("data-lucide", "loader-2");
  runIcon.classList.add("spinner");
  lucide.createIcons();

  updateStatus("running");

  outContent.innerHTML = `
    <div class="placeholder-text">
      <i data-lucide="loader-2" class="spinner" style="width:20px;height:20px;color:var(--text-muted)"></i>
      <span style="margin-top: 8px;">Submitting execution script...</span>
    </div>
  `;
  lucide.createIcons();

  document.getElementById("metaInfo").style.display = "none";
  document.getElementById("copyOutputBtn").style.display = "none";

  try {
    const response = await fetch(`${API_BASE}/submit`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ language, code, input }),
    });

    if (!response.ok) throw new Error("API Server error");

    const data = await response.json();
    pollStatus(data.submission_id);
  } catch (err) {
    outContent.innerHTML = `
      <div class="stderr-box">
        <div class="stderr-header">
          <i data-lucide="alert-triangle" style="width: 14px; height: 14px"></i>
          <span>Execution Error</span>
        </div>
        <pre class="output-pre">Unable to communicate with the compilation microservices gateway.</pre>
      </div>
    `;
    lucide.createIcons();
    showToast("Connection failed", "error");
    resetButton();
  }
}

// Stream status pipeline
async function pollStatus(id) {
  const outContent = document.getElementById("outputContent");
  const evtSource = new EventSource(`${API_BASE}/stream/${id}`);

  evtSource.onmessage = function (event) {
    const data = JSON.parse(event.data);
    updateStatus(data.status);

    if (data.status === "running") {
      outContent.innerHTML = `
        <div class="placeholder-text">
          <i data-lucide="loader-2" class="spinner" style="width:20px;height:20px;color:var(--text-muted)"></i>
          <span style="margin-top: 8px;">Executing sandbox test operations...</span>
        </div>
      `;
      lucide.createIcons();
    }

    if (
      data.status === "completed" ||
      data.status === "error" ||
      data.status === "timeout"
    ) {
      evtSource.close();
      resetButton();

      let hasOutput = false;
      let outputHTML = "";

      if (data.stdout) {
        hasOutput = true;
        outputHTML += `<pre class="output-pre">${escapeHTML(data.stdout)}</pre>`;
      }

      if (data.status === "timeout") {
        hasOutput = true;
        outputHTML += `
          <div class="stderr-box">
            <div class="stderr-header">
              <i data-lucide="clock" style="width:13px;height:13px"></i>
              <span>Timeout limit hit</span>
            </div>
            <pre class="output-pre">The process exceeded runtime sandbox limits (7000ms).</pre>
          </div>
        `;
      } else if (
        data.status === "completed" &&
        !data.stdout &&
        !data.stderr
      ) {
        outputHTML += `
          <div class="placeholder-text">
            <i data-lucide="check-circle" style="width:20px;height:20px;color:var(--success)"></i>
            <span style="margin-top: 8px;">Process completed successfully with no returned output streams.</span>
          </div>
        `;
      }

      if (data.stderr) {
        hasOutput = true;
        outputHTML += `
          <div class="stderr-box">
            <div class="stderr-header">
              <i data-lucide="alert-circle" style="width:13px;height:13px"></i>
              <span>Compilation & Standard Error logs</span>
            </div>
            <pre class="output-pre">${escapeHTML(data.stderr)}</pre>
          </div>
        `;
      }

      outContent.innerHTML = outputHTML;
      lucide.createIcons();

      if (hasOutput && data.stdout) {
        document.getElementById("copyOutputBtn").style.display =
          "inline-flex";
      }

      if (data.execution_time_ms !== null) {
        const meta = document.getElementById("metaInfo");
        meta.innerText = `${data.execution_time_ms} ms`;
        meta.style.display = "inline-flex";
      }

      if (data.status === "completed") {
        showToast("Execution completed successfully", "success");
      } else {
        showToast("Execution halted with exceptions", "error");
      }
    }
  };

  evtSource.onerror = function () {
    evtSource.close();
    resetButton();
    outContent.innerHTML += `
      <div class="stderr-box" style="margin-top: 10px;">
        <div class="stderr-header">
          <i data-lucide="wifi-off" style="width: 14px; height: 14px"></i>
          <span>Runtime offline</span>
        </div>
        <pre class="output-pre">The stream pipe interface timed out unexpectedly.</pre>
      </div>
    `;
    lucide.createIcons();
    showToast("Server link interrupted", "error");
  };
}

function updateStatus(status) {
  const badge = document.getElementById("statusBadge");
  badge.style.display = "inline-block";
  badge.innerText = status;
  badge.className = `status-badge status-${status}`;
}

function resetButton() {
  const btn = document.getElementById("runBtn");
  const icon = document.getElementById("runIcon");
  const text = document.getElementById("runText");

  btn.disabled = false;
  text.innerText = "Run Code";
  icon.setAttribute("data-lucide", "play");
  icon.classList.remove("spinner");
  lucide.createIcons();
}

function copyOutput() {
  const outputPres = document.querySelectorAll(
    "#outputContent .output-pre",
  );
  if (outputPres.length === 0) return;

  const text = outputPres[0].innerText;
  navigator.clipboard
    .writeText(text)
    .then(() => showToast("Copied outputs to clipboard", "success"))
    .catch(() => showToast("Failed to copy outputs", "error"));
}

// CP Custom Template Management system (session persistent)
function getCustomTemplates(lang) {
  const saved = localStorage.getItem(`cp_templates_${lang}`);
  if (saved) {
    try {
      return JSON.parse(saved);
    } catch (e) {
      return {};
    }
  }
  return {};
}

// Save templates helper
function saveCustomTemplates(lang, templates) {
  localStorage.setItem(`cp_templates_${lang}`, JSON.stringify(templates));
}

function populateTemplateSelector() {
  const lang = document.getElementById("language").value;
  const selector = document.getElementById("templateSelector");
  if (!selector) return;

  // Clear existing, keep placeholder
  selector.innerHTML =
    '<option value="" disabled selected hidden>Templates</option>';

  // Add Default option
  const defaultOpt = document.createElement("option");
  defaultOpt.value = "Default";
  defaultOpt.innerText = "Default";
  selector.appendChild(defaultOpt);

  // Add Saved Custom templates
  const templates = getCustomTemplates(lang);
  Object.keys(templates).forEach((name) => {
    const opt = document.createElement("option");
    opt.value = name;
    opt.innerText = name;
    selector.appendChild(opt);
  });
}

function saveAsTemplate() {
  if (!editor) return;
  const lang = document.getElementById("language").value;
  const name = prompt("Enter a name for this custom template:");
  if (!name) return;

  const trimmed = name.trim();
  if (trimmed === "Default" || trimmed === "") {
    showToast("Invalid template name.", "error");
    return;
  }

  const templates = getCustomTemplates(lang);
  templates[trimmed] = editor.getValue();
  saveCustomTemplates(lang, templates);

  populateTemplateSelector();
  document.getElementById("templateSelector").value = trimmed;
  showToast(`Saved template "${trimmed}"!`, "success");
}

function deleteTemplate() {
  const lang = document.getElementById("language").value;
  const selector = document.getElementById("templateSelector");
  const selectedName = selector.value;

  if (!selectedName || selectedName === "Default") {
    showToast("Cannot delete default template.", "warning");
    return;
  }

  if (
    confirm(
      `Are you sure you want to delete the template "${selectedName}"?`,
    )
  ) {
    const templates = getCustomTemplates(lang);
    delete templates[selectedName];
    saveCustomTemplates(lang, templates);

    populateTemplateSelector();
    showToast(`Deleted template "${selectedName}".`, "info");
  }
}

function loadTemplate(name) {
  if (!name || !editor) return;
  const lang = document.getElementById("language").value;

  let code = "";
  if (name === "Default") {
    code = TEMPLATES[lang];
  } else {
    const templates = getCustomTemplates(lang);
    code = templates[name];
  }

  if (code !== undefined) {
    setEditorValue(code);
    saveCode(lang, code);
    showToast(`Loaded template "${name}"`, "success");
  }
}

function escapeHTML(str) {
  return str.replace(
    /[&<>'"]/g,
    (tag) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "'": "&#39;",
        '"': "&quot;",
      })[tag] || tag,
  );
}
