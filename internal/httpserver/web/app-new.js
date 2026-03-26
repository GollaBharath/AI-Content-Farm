const byId = (id) => document.getElementById(id);

// Element references
const statusEl = byId("status");
const jobsEl = byId("jobs");
const refreshBtn = byId("refresh");
const collapseJobsBtn = byId("collapse-jobs");
const collapseSettingsBtn = byId("collapse-settings");

const presetSelect = byId("preset-select");
const presetSettings = byId("preset-settings");

const scriptModeAiBtn = byId("script-mode-ai");
const scriptModeManualBtn = byId("script-mode-manual");
const aiScriptSection = byId("ai-script-section");
const manualScriptSection = byId("manual-script-section");
const generateScriptBtn = byId("generate-script");
const generatedScriptView = byId("generated-script-view");

const scriptOverrideEl = byId("script-override");
const manualScriptEl = byId("manual-script");
const renderJobBtn = byId("render-job");
const renderJobManualBtn = byId("render-job-manual");

const topicEl = byId("topic");
const promptEl = byId("prompt");
const languageEl = byId("language");
const voiceEl = byId("voice");
const bgSelect = byId("background-video");
const orientationEl = byId("orientation");
const customSizeRow = byId("custom-size-row");
const customWidthEl = byId("custom-width");
const customHeightEl = byId("custom-height");

const uploadAreaEl = byId("upload-area");
const uploadInput = byId("video-upload");
const uploadVideoBtn = byId("upload-videos");
const youtubeURLInput = byId("youtube-url");
const importYouTubeBtn = byId("import-youtube");

const generatedVideosEl = byId("generated-videos");
const uploadedVideosEl = byId("uploaded-videos");

const videoTabBtns = document.querySelectorAll(".video-tab-btn");
const videoTabContents = document.querySelectorAll(".video-tab-content");

const advancedSettingsEl = byId("advanced-settings");

const inputDirEl = byId("input-dir");
const outputDirEl = byId("output-dir");
const defaultOrientationEl = byId("default-orientation");
const defaultLanguageEl = byId("default-language");
const defaultVoiceEl = byId("default-voice");
const voicePreviewTextEl = byId("voice-preview-text");
const previewVoiceBtn = byId("preview-voice");
const voicePreviewPlayer = byId("voice-preview-player");
const saveSettingsBtn = byId("save-settings");
const saveVoiceSettingsBtn = byId("save-voice-settings");
const voicesListEl = byId("voices-list");
const savePresetBtn = byId("save-preset");

let availableVoices = [];
let savedPresets = [];
let previewObjectURL = "";

// Setup functions
function setStatus(text, type = "info") {
	statusEl.textContent = text || "";
	statusEl.className = type ? `status-message ${type}` : "status-message";
}

function setupPresetSelection() {
	presetSelect.addEventListener("change", () => {
		const presetIdx = parseInt(presetSelect.value, 10);
		if (presetIdx === "" || isNaN(presetIdx)) {
			presetSettings.style.display = "block";
			clearPresetForm();
		} else {
			const preset = savedPresets[presetIdx];
			loadPresetIntoForm(preset);
			presetSettings.style.display = "none";
		}
	});
}

function setupPresetSettings() {
	// Expand settings when no preset is selected or create new is clicked
	const currentValue = presetSelect.value;
	if (currentValue === "") {
		presetSettings.style.display = "block";
	}
}

function setupScriptMode() {
	scriptModeAiBtn.addEventListener("click", () => {
		scriptModeAiBtn.classList.add("active");
		scriptModeManualBtn.classList.remove("active");
		aiScriptSection.style.display = "block";
		manualScriptSection.style.display = "none";
		generatedScriptView.style.display = "none";
	});

	scriptModeManualBtn.addEventListener("click", () => {
		scriptModeManualBtn.classList.add("active");
		scriptModeAiBtn.classList.remove("active");
		aiScriptSection.style.display = "none";
		manualScriptSection.style.display = "block";
	});
}

function setupVideoTabs() {
	videoTabBtns.forEach((btn) => {
		btn.addEventListener("click", () => {
			const tabName = btn.getAttribute("data-video-tab");
			videoTabBtns.forEach((b) => b.classList.remove("active"));
			btn.classList.add("active");
			videoTabContents.forEach((content) => content.classList.remove("active"));
			byId(`video-tab-${tabName}`).classList.add("active");
		});
	});
}

function setupCollapsible() {
	collapseJobsBtn.addEventListener("click", () => {
		jobsEl.style.display = jobsEl.style.display === "none" ? "grid" : "none";
		collapseJobsBtn.textContent = jobsEl.style.display === "none" ? "+" : "−";
	});

	collapseSettingsBtn.addEventListener("click", () => {
		advancedSettingsEl.style.display =
			advancedSettingsEl.style.display === "none" ? "block" : "none";
		collapseSettingsBtn.textContent =
			advancedSettingsEl.style.display === "none" ? "+" : "−";
	});
}

function setupUploadArea() {
	uploadAreaEl.addEventListener("click", () => uploadInput.click());

	uploadAreaEl.addEventListener("dragover", (e) => {
		e.preventDefault();
		uploadAreaEl.style.borderColor = "#000";
		uploadAreaEl.style.background = "#f0f0f0";
	});

	uploadAreaEl.addEventListener("dragleave", () => {
		uploadAreaEl.style.borderColor = "var(--border)";
		uploadAreaEl.style.background = "var(--bg-secondary)";
	});

	uploadAreaEl.addEventListener("drop", (e) => {
		e.preventDefault();
		uploadAreaEl.style.borderColor = "var(--border)";
		uploadAreaEl.style.background = "var(--bg-secondary)";
		uploadInput.files = e.dataTransfer.files;
	});
}

// Voice functions
function normalizeVoice(v) {
	return {
		key: v.key || v.name || "",
		name: v.name || v.key || "Unnamed",
		language_code: v.language_code || "",
		quality: v.quality || "",
	};
}

async function loadVoices() {
	try {
		const resp = await fetch("/api/voices");
		if (!resp.ok) {
			const data = await resp.json().catch(() => ({}));
			throw new Error(data.error || "Failed to load Piper voices");
		}
		const data = await resp.json();
		availableVoices = (data.voices || []).map(normalizeVoice);

		const languageSet = new Set();
		for (const voice of availableVoices) {
			if (voice.language_code) {
				languageSet.add(voice.language_code);
			}
		}
		for (const language of data.languages || []) {
			if (language) {
				languageSet.add(language);
			}
		}

		const languages = Array.from(languageSet).sort();
		renderLanguageDropdown(languageEl, languages, "Auto");
		renderLanguageDropdown(defaultLanguageEl, languages, "Auto");
		refreshVoiceDropdowns();
		renderVoicesList();
	} catch (e) {
		setStatus("Failed to load voices: " + e.message, "error");
	}
}

function renderLanguageDropdown(selectEl, languages, defaultLabel) {
	const prev = selectEl.value;
	selectEl.innerHTML = "";
	selectEl.appendChild(new Option(defaultLabel, ""));
	languages.forEach((lang) => selectEl.appendChild(new Option(lang, lang)));
	if (prev && languages.includes(prev)) {
		selectEl.value = prev;
	}
}

function filteredVoicesByLanguage(language) {
	if (!language) {
		return availableVoices;
	}
	return availableVoices.filter((v) => v.language_code === language);
}

function refreshVoiceDropdowns() {
	const mainLang = languageEl.value.trim();
	const defaultLang = defaultLanguageEl.value.trim();
	renderVoiceDropdown(voiceEl, mainLang, "Default");
	renderVoiceDropdown(defaultVoiceEl, defaultLang, "None");
	renderVoicesList();
}

function renderVoiceDropdown(selectEl, language, emptyLabel) {
	const prev = selectEl.value;
	const voices = filteredVoicesByLanguage(language);
	selectEl.innerHTML = "";
	selectEl.appendChild(new Option(emptyLabel, ""));
	voices.forEach((voice) => {
		const suffix = voice.quality ? ` (${voice.quality})` : "";
		selectEl.appendChild(new Option(`${voice.name}${suffix}`, voice.key));
	});
	if (prev && voices.some((v) => v.key === prev)) {
		selectEl.value = prev;
	}
}

function renderVoicesList() {
	const voices = filteredVoicesByLanguage(defaultLanguageEl.value.trim());
	if (!voices.length) {
		voicesListEl.innerHTML =
			'<p class="empty-state">No voices for this language</p>';
		return;
	}

	voicesListEl.innerHTML = voices
		.map(
			(voice) => `
		<div class="list-item">
			<div>
				<div class="list-item-name">${voice.name}</div>
				<div class="list-item-meta">${voice.language_code || "n/a"}${voice.quality ? ` | ${voice.quality}` : ""}</div>
			</div>
			<div class="list-actions">
				<button class="list-item-action" data-use="${voice.key}">Use</button>
				<button class="list-item-action" data-preview="${voice.key}">Preview</button>
			</div>
		</div>
	`,
		)
		.join("");

	voicesListEl.querySelectorAll("button[data-use]").forEach((btn) => {
		btn.addEventListener("click", () => {
			const key = btn.getAttribute("data-use") || "";
			defaultVoiceEl.value = key;
			voiceEl.value = key;
			setStatus("Voice selected.", "success");
		});
	});

	voicesListEl.querySelectorAll("button[data-preview]").forEach((btn) => {
		btn.addEventListener("click", () => {
			const key = btn.getAttribute("data-preview") || "";
			previewVoice(key).catch((e) => setStatus(e.message, "error"));
		});
	});
}

async function previewVoice(voiceKey) {
	const text =
		voicePreviewTextEl.value.trim() || "This is a quick Piper voice preview.";
	const language = defaultLanguageEl.value.trim() || languageEl.value.trim();
	previewVoiceBtn.disabled = true;
	setStatus("Generating voice preview...");
	try {
		const resp = await fetch("/api/voices/preview", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({
				text,
				voice: voiceKey || defaultVoiceEl.value,
				language,
			}),
		});
		if (!resp.ok) {
			const data = await resp.json().catch(() => ({}));
			throw new Error(data.error || "Voice preview failed");
		}
		const blob = await resp.blob();
		if (previewObjectURL) {
			URL.revokeObjectURL(previewObjectURL);
		}
		previewObjectURL = URL.createObjectURL(blob);
		voicePreviewPlayer.src = previewObjectURL;
		await voicePreviewPlayer.play().catch(() => {});
		setStatus("Voice preview ready.", "success");
	} finally {
		previewVoiceBtn.disabled = false;
	}
}

// Preset functions
function clearPresetForm() {
	topicEl.value = "";
	promptEl.value = "";
	secondsEl.value = "60";
	languageEl.value = "";
	voiceEl.value = "";
	orientationEl.value = "portrait";
	customWidthEl.value = "";
	customHeightEl.value = "";
	bgSelect.value = "";
	scriptOverrideEl.value = "";
	manualScriptEl.value = "";
	scriptModeAiBtn.classList.add("active");
	scriptModeManualBtn.classList.remove("active");
	aiScriptSection.style.display = "block";
	manualScriptSection.style.display = "none";
	generatedScriptView.style.display = "none";
}

function loadPresetIntoForm(preset) {
	topicEl.value = preset.topic || "";
	promptEl.value = preset.prompt || "";
	languageEl.value = preset.language || "";
	refreshVoiceDropdowns();
	voiceEl.value = preset.voice || "";
	orientationEl.value = preset.orientation || "portrait";
	customWidthEl.value = preset.custom_width || "";
	customHeightEl.value = preset.custom_height || "";
	bgSelect.value = preset.background_video || "";
	updateCustomSizeVisibility();
	setStatus(`Preset "${preset.name}" loaded.`, "success");
}

function renderPresetsList() {
	presetSelect.innerHTML =
		'<option value="">-- Create New / One-Time --</option>';
	savedPresets.forEach((preset, idx) => {
		presetSelect.appendChild(new Option(preset.name, idx));
	});
}

async function savePreset() {
	const presetName = prompt("Enter preset name:");
	if (!presetName) return;
	const name = presetName.trim();
	if (!name) return;

	const preset = {
		name,
		topic: topicEl.value.trim(),
		prompt: promptEl.value.trim(),
		script_override: scriptOverrideEl.value.trim(),
		voice: voiceEl.value,
		language: languageEl.value,
		orientation: orientationEl.value,
		custom_width: Number(customWidthEl.value || 0),
		custom_height: Number(customHeightEl.value || 0),
		background_video: bgSelect.value,
	};

	savedPresets = savedPresets.filter((p) => p.name !== name);
	savedPresets.unshift(preset);

	try {
		const resp = await fetch("/api/settings", {
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ prompt_presets: savedPresets }),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) {
			throw new Error(data.error || "Failed to save preset");
		}
		savedPresets = Array.isArray(data.prompt_presets)
			? data.prompt_presets
			: [];
		renderPresetsList();
		setStatus("Preset saved.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	}
}

// Script and rendering
function collectJobPayload() {
	return {
		topic: topicEl.value.trim(),
		prompt: promptEl.value.trim(),
		script_override:
			scriptOverrideEl.value.trim() || manualScriptEl.value.trim(),
		voice: voiceEl.value.trim(),
		language: languageEl.value.trim(),
		orientation: orientationEl.value,
		custom_width: Number(customWidthEl.value || 0),
		custom_height: Number(customHeightEl.value || 0),
		background_video: bgSelect.value,
	};
}

async function generateScriptDraft() {
	setStatus("Generating script draft...");
	generateScriptBtn.disabled = true;
	try {
		const resp = await fetch("/v1/scripts/generate", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(collectJobPayload()),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) throw new Error(data.error || "Script generation failed");
		scriptOverrideEl.value = data.script || "";
		generatedScriptView.style.display = "block";
		setStatus("Draft ready. Review and edit if needed.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		generateScriptBtn.disabled = false;
	}
}

async function renderJobRequest() {
	setStatus("Queueing job...");
	renderJobBtn.disabled = true;
	renderJobManualBtn.disabled = true;
	try {
		const resp = await fetch("/v1/jobs", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(collectJobPayload()),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) throw new Error(data.error || "Failed to queue job");
		setStatus("Job queued successfully.", "success");
		await loadJobs();
		clearPresetForm();
		presetSelect.value = "";
		presetSettings.style.display = "block";
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		renderJobBtn.disabled = false;
		renderJobManualBtn.disabled = false;
	}
}

// Jobs
function badgeClass(status) {
	const statusLower = status?.toLowerCase() || "queued";
	return `badge-${statusLower}`;
}

function renderJob(job) {
	const el = document.createElement("article");
	el.className = "job";

	const title = job.request?.topic || "Untitled";
	const script = job.script || "";
	const err = job.error_message
		? `<div class="job-error">${job.error_message}</div>`
		: "";
	const output = job.output_path
		? `<a href="${job.output_path}" target="_blank" rel="noopener" class="job-link">Open Video</a>`
		: "";

	el.innerHTML = `
		<div class="job-meta">
			<span>${new Date(job.created_at).toLocaleString()}</span>
			<span class="badge ${badgeClass(job.status)}">${job.status?.toUpperCase() || "QUEUED"}</span>
		</div>
		<h3>${title}</h3>
		<pre>${script || "No script"}</pre>
		${err}
		${output}
	`;
	return el;
}

async function loadJobs() {
	try {
		const resp = await fetch("/v1/jobs");
		if (!resp.ok) throw new Error("Failed to load jobs");
		const jobs = await resp.json();
		jobsEl.innerHTML = "";

		if (!jobs || jobs.length === 0) {
			jobsEl.innerHTML = '<p class="empty-state">No jobs yet</p>';
			return;
		}

		jobs.forEach((job) => jobsEl.appendChild(renderJob(job)));
	} catch (e) {
		setStatus(e.message, "error");
	}
}

// Videos
async function loadVideos() {
	try {
		const resp = await fetch("/api/videos");
		if (!resp.ok) throw new Error("Failed to load videos");
		const videos = await resp.json();
		bgSelect.innerHTML = '<option value="">Auto (Random)</option>';
		if (Array.isArray(videos) && videos.length > 0) {
			videos.forEach((v) => bgSelect.appendChild(new Option(v.name, v.name)));
		}
		renderVideosList(videos || []);
	} catch (e) {
		setStatus(e.message, "error");
	}
}

function renderVideosList(videos) {
	if (!videos || videos.length === 0) {
		uploadedVideosEl.innerHTML =
			'<p class="empty-state">No uploaded videos</p>';
		return;
	}
	uploadedVideosEl.innerHTML = videos
		.map(
			(v) => `
		<div class="video-item">
			<span class="video-item-name">${v.name}</span>
			<div class="video-item-actions">
				<button class="video-action-btn rename" data-video="${v.name}">Rename</button>
				<button class="video-action-btn download" data-video="${v.name}">Download</button>
				<button class="video-action-btn delete" data-video="${v.name}">Delete</button>
			</div>
		</div>
	`,
		)
		.join("");

	uploadedVideosEl.querySelectorAll(".video-action-btn").forEach((btn) => {
		const videoName = btn.getAttribute("data-video");
		if (btn.classList.contains("rename")) {
			btn.addEventListener("click", () => renameVideo(videoName));
		} else if (btn.classList.contains("download")) {
			btn.addEventListener("click", () => downloadVideo(videoName));
		} else if (btn.classList.contains("delete")) {
			btn.addEventListener("click", () => deleteVideo(videoName));
		}
	});
}

async function renameVideo(oldName) {
	const newName = prompt("Enter new name:", oldName);
	if (!newName || newName === oldName) return;
	setStatus("Renaming video...");
	try {
		const resp = await fetch("/api/videos/rename", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ old_name: oldName, new_name: newName }),
		});
		if (!resp.ok) {
			const data = await resp.json().catch(() => ({}));
			throw new Error(data.error || "Rename failed");
		}
		await loadVideos();
		setStatus("Video renamed.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	}
}

async function downloadVideo(videoName) {
	window.location.href = `/api/videos/download?name=${encodeURIComponent(videoName)}`;
}

async function deleteVideo(videoName) {
	if (!confirm(`Delete "${videoName}"?`)) return;
	setStatus("Deleting video...");
	try {
		const resp = await fetch("/api/videos/delete", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ name: videoName }),
		});
		if (!resp.ok) {
			const data = await resp.json().catch(() => ({}));
			throw new Error(data.error || "Delete failed");
		}
		await loadVideos();
		setStatus("Video deleted.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	}
}

async function uploadVideos() {
	if (!uploadInput.files || !uploadInput.files.length) {
		setStatus("Select one or more files first.", "error");
		return;
	}
	setStatus("Uploading videos...");
	uploadVideoBtn.disabled = true;
	try {
		const fd = new FormData();
		for (const f of uploadInput.files) fd.append("videos", f);
		const resp = await fetch("/api/videos/upload", {
			method: "POST",
			body: fd,
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) throw new Error(data.error || "Upload failed");
		setStatus(
			`Uploaded ${data.uploaded?.length || uploadInput.files.length} file(s).`,
			"success",
		);
		uploadInput.value = "";
		await loadVideos();
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		uploadVideoBtn.disabled = false;
	}
}

async function importYouTubeVideo() {
	const url = (youtubeURLInput.value || "").trim();
	if (!url) {
		setStatus("Paste a YouTube URL first.", "error");
		return;
	}

	importYouTubeBtn.disabled = true;
	setStatus("Importing and splitting YouTube video...");
	try {
		const resp = await fetch("/api/videos/import-youtube", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ url }),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) {
			throw new Error(data.error || "YouTube import failed");
		}
		youtubeURLInput.value = "";
		await loadVideos();
		const compressed = Number(data.clips_compressed || 0);
		setStatus(
			`Imported ${data.clips_created || 0} full 56s clips (tail discarded).${compressed > 0 ? ` Compressed ${compressed} oversized clip(s).` : ""}`,
			"success",
		);
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		importYouTubeBtn.disabled = false;
	}
}

// Settings
async function loadSettings() {
	try {
		const resp = await fetch("/api/settings");
		if (!resp.ok) throw new Error("Failed to load settings");
		const s = await resp.json();

		inputDirEl.value = s.input_videos_dir || "";
		outputDirEl.value = s.output_videos_dir || "";
		defaultOrientationEl.value = s.default_video_orientation || "portrait";
		defaultLanguageEl.value = s.default_language || "";
		languageEl.value = s.default_language || "";
		refreshVoiceDropdowns();
		defaultVoiceEl.value = s.default_voice || "";
		if (!voiceEl.value.trim() && s.default_voice) {
			voiceEl.value = s.default_voice;
		}
		savedPresets = Array.isArray(s.prompt_presets) ? s.prompt_presets : [];
		renderPresetsList();
		updateCustomSizeVisibility();
	} catch (e) {
		setStatus(e.message, "error");
	}
}

async function saveSettings() {
	setStatus("Saving settings...");
	saveSettingsBtn.disabled = true;
	try {
		const payload = {
			input_videos_dir: inputDirEl.value.trim(),
			output_videos_dir: outputDirEl.value.trim(),
			default_video_orientation: defaultOrientationEl.value,
			default_voice: defaultVoiceEl.value.trim(),
			default_language: defaultLanguageEl.value.trim(),
		};
		const resp = await fetch("/api/settings", {
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(payload),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok) throw new Error(data.error || "Failed to save settings");
		setStatus("Settings saved.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		saveSettingsBtn.disabled = false;
	}
}

async function saveVoiceSettings() {
	saveVoiceSettingsBtn.disabled = true;
	try {
		const payload = {
			default_voice: defaultVoiceEl.value.trim(),
			default_language: defaultLanguageEl.value.trim(),
		};
		const resp = await fetch("/api/settings", {
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(payload),
		});
		const data = await resp.json().catch(() => ({}));
		if (!resp.ok)
			throw new Error(data.error || "Failed to save voice settings");
		setStatus("Voice settings saved.", "success");
	} catch (e) {
		setStatus(e.message, "error");
	} finally {
		saveVoiceSettingsBtn.disabled = false;
	}
}

function updateCustomSizeVisibility() {
	customSizeRow.style.display =
		orientationEl.value === "custom" ? "grid" : "none";
}

// Event listeners
generateScriptBtn?.addEventListener("click", () => generateScriptDraft());
renderJobBtn?.addEventListener("click", () => renderJobRequest());
renderJobManualBtn?.addEventListener("click", () => renderJobRequest());
saveSettingsBtn?.addEventListener("click", () => saveSettings());
saveVoiceSettingsBtn?.addEventListener("click", () => saveVoiceSettings());
savePresetBtn?.addEventListener("click", () =>
	savePreset().catch((e) => setStatus(e.message, "error")),
);
previewVoiceBtn?.addEventListener("click", () =>
	previewVoice(defaultVoiceEl.value || voiceEl.value).catch((e) =>
		setStatus(e.message, "error"),
	),
);
uploadVideoBtn?.addEventListener("click", () => uploadVideos());
importYouTubeBtn?.addEventListener("click", () => importYouTubeVideo());
refreshBtn?.addEventListener("click", () =>
	loadJobs().catch((e) => setStatus(e.message, "error")),
);
orientationEl?.addEventListener("change", updateCustomSizeVisibility);
languageEl?.addEventListener("change", refreshVoiceDropdowns);
defaultLanguageEl?.addEventListener("change", () => {
	if (!languageEl.value) {
		languageEl.value = defaultLanguageEl.value;
	}
	refreshVoiceDropdowns();
});

// Bootstrap
async function boot() {
	setupPresetSelection();
	setupPresetSettings();
	setupScriptMode();
	setupVideoTabs();
	setupCollapsible();
	setupUploadArea();
	try {
		await loadVoices();
		await loadSettings();
		await Promise.all([loadVideos(), loadJobs()]);
		setStatus("Ready to create content.", "success");
	} catch (e) {
		setStatus(e.message || "Failed to initialize", "error");
	}
}

boot();
setInterval(() => loadJobs().catch(() => {}), 8000);
