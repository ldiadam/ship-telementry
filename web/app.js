// API Base URL
const API_BASE = 'http://localhost:31180';

// Global state
let selectedVesselId = null;
let vessels = [];

// DOM Elements
const uploadForm = document.getElementById('uploadForm');
const fileInput = document.getElementById('fileInput');
const dropZone = document.getElementById('dropZone');
const fileInfo = document.getElementById('fileInfo');
const uploadProgress = document.getElementById('uploadProgress');
const progressBar = document.getElementById('progressBar');
const uploadResult = document.getElementById('uploadResult');
const vesselsList = document.getElementById('vesselsList');
const refreshVessels = document.getElementById('refreshVessels');
const telemetrySection = document.getElementById('telemetrySection');
const streamSelect = document.getElementById('streamSelect');
const loadTelemetry = document.getElementById('loadTelemetry');
const telemetryData = document.getElementById('telemetryData');

// Initialize app
document.addEventListener('DOMContentLoaded', function() {
    setupEventListeners();
    loadVessels();
});

function setupEventListeners() {
    // File upload
    dropZone.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', handleFileSelect);
    
    // Drag and drop
    dropZone.addEventListener('dragover', handleDragOver);
    dropZone.addEventListener('drop', handleDrop);
    
    // Form submission
    uploadForm.addEventListener('submit', handleUpload);
    
    // Vessels
    refreshVessels.addEventListener('click', loadVessels);
    
    // Telemetry
    loadTelemetry.addEventListener('click', handleLoadTelemetry);
}

function handleDragOver(e) {
    e.preventDefault();
    dropZone.classList.add('dragover');
}

function handleDrop(e) {
    e.preventDefault();
    dropZone.classList.remove('dragover');
    
    const files = e.dataTransfer.files;
    if (files.length > 0) {
        fileInput.files = files;
        handleFileSelect();
    }
}

// Add drag leave handler
dropZone.addEventListener('dragleave', (e) => {
    e.preventDefault();
    dropZone.classList.remove('dragover');
});

function handleFileSelect() {
    const file = fileInput.files[0];
    if (file) {
        fileInfo.querySelector('p').textContent = `Selected: ${file.name} (${formatFileSize(file.size)})`;
        fileInfo.classList.remove('hidden');
    }
}

function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

async function handleUpload(e) {
    e.preventDefault();
    
    const file = fileInput.files[0];
    const imo = document.getElementById('imo').value.trim();
    const vesselName = document.getElementById('vesselName').value.trim();
    const periodStart = document.getElementById('periodStart').value;
    
    // Validation
    if (!file) {
        showError('Please select an XLSX file');
        return;
    }
    
    if (!imo && !vesselName) {
        showError('Please provide either IMO number or vessel name');
        return;
    }
    
    // Show progress
    uploadProgress.classList.remove('hidden');
    uploadResult.classList.add('hidden');
    document.getElementById('uploadBtn').disabled = true;
    
    try {
        const formData = new FormData();
        formData.append('file', file);
        
        let url = `${API_BASE}/ingest/xlsx`;
        const params = new URLSearchParams();
        
        if (imo) params.append('imo', imo);
        if (vesselName) params.append('vessel_name', vesselName);
        if (periodStart) {
            const isoDate = new Date(periodStart).toISOString();
            params.append('period_start', isoDate);
        }
        
        if (params.toString()) {
            url += '?' + params.toString();
        }
        
        const response = await fetch(url, {
            method: 'POST',
            body: formData
        });
        
        const result = await response.json();
        
        if (response.ok) {
            showSuccess(result);
            uploadForm.reset();
            fileInfo.classList.add('hidden');
            loadVessels(); // Refresh vessels list
        } else {
            showError(result.error || 'Upload failed');
        }
        
    } catch (error) {
        showError('Network error: ' + error.message);
    } finally {
        uploadProgress.classList.add('hidden');
        document.getElementById('uploadBtn').disabled = false;
    }
}

function showSuccess(result) {
    const html = `
        <div class="modern-alert-success">
            <div class="flex items-center mb-3">
                <i class="fas fa-check-circle text-green-600 mr-2"></i>
                <h3 class="font-semibold">Upload Successful</h3>
            </div>
            <div class="space-y-2">
                <p><strong>Status:</strong> ${result.status}</p>
                <p><strong>Upload ID:</strong> ${result.upload_id}</p>
                <p><strong>Vessel ID:</strong> ${result.vessel_id}</p>
                
                ${result.rows_inserted ? `
                <div class="mt-3">
                    <p class="font-medium mb-2">Rows Inserted:</p>
                    <div class="grid grid-cols-2 gap-2 text-sm">
                        ${Object.entries(result.rows_inserted).map(([stream, count]) => 
                            `<div class="flex justify-between">
                                <span>${getStreamIcon(stream)} ${stream}:</span>
                                <span class="font-medium">${count} rows</span>
                            </div>`
                        ).join('')}
                    </div>
                </div>
                ` : ''}
                
                ${result.warnings && result.warnings.length > 0 ? `
                <div class="mt-3 p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
                    <p class="font-medium text-yellow-800 mb-2">Warnings:</p>
                    <ul class="text-sm text-yellow-700 space-y-1">
                        ${result.warnings.map(warning => `<li>• ${warning}</li>`).join('')}
                    </ul>
                </div>
                ` : ''}
            </div>
        </div>
    `;
    
    uploadResult.innerHTML = html;
    uploadResult.classList.remove('hidden');
}

function showError(message) {
    const html = `
        <div class="modern-alert-error">
            <div class="flex items-center mb-2">
                <i class="fas fa-exclamation-circle text-red-600 mr-2"></i>
                <h3 class="font-semibold">Upload Failed</h3>
            </div>
            <p>${message}</p>
        </div>
    `;
    
    uploadResult.innerHTML = html;
    uploadResult.classList.remove('hidden');
}

async function loadVessels() {
    try {
        vesselsList.innerHTML = `
            <div class="text-center py-8 text-gray-500">
                <i class="fas fa-spinner fa-spin text-2xl mb-2"></i>
                <p>Loading vessels...</p>
            </div>
        `;
        
        const response = await fetch(`${API_BASE}/vessels`);
        vessels = await response.json();
        
        if (vessels.length === 0) {
            vesselsList.innerHTML = `
                <div class="text-center py-8 text-gray-500">
                    <i class="fas fa-ship text-4xl mb-2"></i>
                    <p>No vessels found</p>
                    <p class="text-sm">Upload telemetry data to get started</p>
                </div>
            `;
            return;
        }
        
        const html = vessels.map(vessel => `
            <div class="vessel-card" data-vessel-id="${vessel.id}">
                <div class="flex flex-col lg:flex-row lg:items-center justify-between space-y-4 lg:space-y-0">
                    <div class="flex-1">
                        <div class="flex items-center space-x-4 mb-3">
                            <div class="bg-blue-100 p-3 rounded-full">
                                <i class="fas fa-ship text-blue-600 text-xl"></i>
                            </div>
                            <div>
                                <h3 class="text-xl font-bold text-gray-900 mb-1">${vessel.name}</h3>
                                <div class="vessel-details space-y-1">
                                    ${vessel.imo ? `<div><strong>IMO:</strong> ${vessel.imo}</div>` : ''}
                                    <div class="flex flex-wrap gap-4 text-sm">
                                        ${vessel.flag ? `<span><strong>Flag:</strong> ${vessel.flag}</span>` : ''}
                                        ${vessel.type ? `<span><strong>Type:</strong> ${vessel.type}</span>` : ''}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="lg:text-right">
                        <div class="latest-data bg-gray-50 p-4 rounded-lg border border-gray-200">
                            <p class="font-semibold text-gray-800 mb-2">
                                <i class="fas fa-clock mr-2 text-blue-600"></i>
                                Latest Telemetry Data
                            </p>
                            ${vessel.latest && Object.keys(vessel.latest).length > 0 ? 
                                `<div class="space-y-1">
                                    ${Object.entries(vessel.latest).map(([stream, timestamp]) => 
                                        `<div class="flex justify-between items-center">
                                            <span class="capitalize font-medium">${getStreamIcon(stream)} ${stream}:</span>
                                            <span class="text-sm text-gray-600 ml-2">${formatTimestamp(timestamp)}</span>
                                        </div>`
                                    ).join('')}
                                </div>` 
                                : '<div class="text-gray-500 italic">No telemetry data available</div>'
                            }
                        </div>
                    </div>
                </div>
            </div>
        `).join('');
        
        vesselsList.innerHTML = html;
        
        // Add click handlers
        document.querySelectorAll('.vessel-card').forEach(card => {
            card.addEventListener('click', () => {
                const vesselId = card.dataset.vesselId;
                selectVessel(vesselId);
            });
        });
        
    } catch (error) {
        vesselsList.innerHTML = `
            <div class="text-center py-8 text-red-500">
                <i class="fas fa-exclamation-triangle text-2xl mb-2"></i>
                <p>Error loading vessels: ${error.message}</p>
            </div>
        `;
    }
}

function selectVessel(vesselId) {
    selectedVesselId = vesselId;
    
    // Update UI
    document.querySelectorAll('.vessel-card').forEach(card => {
        card.classList.remove('bg-blue-50', 'border-blue-300');
    });
    
    const selectedCard = document.querySelector(`[data-vessel-id="${vesselId}"]`);
    selectedCard.classList.add('bg-blue-50', 'border-blue-300');
    
    // Show telemetry section
    telemetrySection.classList.remove('hidden');
    telemetryData.innerHTML = '<p class="text-gray-500 text-center py-4">Select a stream to view telemetry data</p>';
}

async function handleLoadTelemetry() {
    if (!selectedVesselId) {
        alert('Please select a vessel first');
        return;
    }
    
    const stream = streamSelect.value;
    if (!stream) {
        alert('Please select a telemetry stream');
        return;
    }
    
    try {
        telemetryData.innerHTML = `
            <div class="text-center py-8 text-gray-500">
                <i class="fas fa-spinner fa-spin text-2xl mb-2"></i>
                <p>Loading ${stream} data...</p>
            </div>
        `;
        
        const response = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=${stream}&limit=50`);
        const result = await response.json();
        
        if (result.items && result.items.length > 0) {
            displayTelemetryData(result.items, stream);
        } else {
            telemetryData.innerHTML = `
                <div class="text-center py-8 text-gray-500">
                    <i class="fas fa-chart-line text-4xl mb-2"></i>
                    <p>No ${stream} data found</p>
                </div>
            `;
        }
        
    } catch (error) {
        telemetryData.innerHTML = `
            <div class="text-center py-8 text-red-500">
                <i class="fas fa-exclamation-triangle text-2xl mb-2"></i>
                <p>Error loading telemetry data: ${error.message}</p>
            </div>
        `;
    }
}

function displayTelemetryData(items, stream) {
    const headers = getStreamHeaders(stream);
    const streamIcon = getStreamIcon(stream);
    const streamTitle = stream.charAt(0).toUpperCase() + stream.slice(1);
    
    const html = `
        <div class="modern-table">
            <div class="px-6 py-4 border-b border-gray-200 bg-gray-50">
                <h3 class="text-lg font-semibold flex items-center">
                    <span class="text-2xl mr-3">${streamIcon}</span>
                    ${streamTitle} Telemetry Data
                    <span class="ml-3 text-sm font-normal text-gray-600">(${items.length} records)</span>
                </h3>
            </div>
            
            <div class="overflow-x-auto">
                <table class="min-w-full">
                    <thead>
                        <tr>
                            ${headers.map(header => `
                                <th>${header.label}</th>
                            `).join('')}
                        </tr>
                    </thead>
                    <tbody>
                        ${items.map((item, index) => `
                            <tr>
                                ${headers.map(header => `
                                    <td>${formatCellValue(item[header.key], header.type)}</td>
                                `).join('')}
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
            
            <div class="px-6 py-4 bg-gray-50 border-t border-gray-200">
                <div class="flex items-center justify-between text-sm">
                    <div class="flex items-center space-x-4">
                        <span class="flex items-center">
                            <i class="fas fa-database mr-2 text-blue-600"></i>
                            Showing ${items.length} records
                        </span>
                        <span class="flex items-center">
                            <i class="fas fa-sort-amount-down mr-2 text-blue-600"></i>
                            Sorted by timestamp (latest first)
                        </span>
                    </div>
                    <div class="text-xs">
                        Last updated: ${new Date().toLocaleString()}
                    </div>
                </div>
            </div>
        </div>
    `;
    
    telemetryData.innerHTML = html;
}

function getStreamHeaders(stream) {
    const commonHeaders = [
        { key: 'ts', label: 'Timestamp', type: 'datetime' },
    ];
    
    const streamHeaders = {
        location: [
            { key: 'latitude', label: 'Latitude', type: 'number' },
            { key: 'longitude', label: 'Longitude', type: 'number' },
            { key: 'course_degrees', label: 'Course (°)', type: 'number' },
            { key: 'speed_knots', label: 'Speed (kn)', type: 'number' },
            { key: 'status', label: 'Status', type: 'text' },
        ],
        engines: [
            { key: 'engine_no', label: 'Engine #', type: 'number' },
            { key: 'rpm', label: 'RPM', type: 'number' },
            { key: 'temp_c', label: 'Temp (°C)', type: 'number' },
            { key: 'oil_pressure_bar', label: 'Oil Pressure (bar)', type: 'number' },
            { key: 'alarms', label: 'Alarms', type: 'text' },
        ],
        fuel: [
            { key: 'tank_no', label: 'Tank #', type: 'number' },
            { key: 'level_percent', label: 'Level (%)', type: 'number' },
            { key: 'volume_liters', label: 'Volume (L)', type: 'number' },
            { key: 'temp_c', label: 'Temp (°C)', type: 'number' },
        ],
        generators: [
            { key: 'gen_no', label: 'Gen #', type: 'number' },
            { key: 'load_kw', label: 'Load (kW)', type: 'number' },
            { key: 'voltage_v', label: 'Voltage (V)', type: 'number' },
            { key: 'frequency_hz', label: 'Frequency (Hz)', type: 'number' },
            { key: 'fuel_rate_lph', label: 'Fuel Rate (L/h)', type: 'number' },
        ],
        cctv: [
            { key: 'cam_id', label: 'Camera ID', type: 'text' },
            { key: 'status', label: 'Status', type: 'text' },
            { key: 'uptime_percent', label: 'Uptime (%)', type: 'number' },
        ],
        impact: [
            { key: 'sensor_id', label: 'Sensor ID', type: 'text' },
            { key: 'accel_g', label: 'Accel (g)', type: 'number' },
            { key: 'shock_g', label: 'Shock (g)', type: 'number' },
            { key: 'notes', label: 'Notes', type: 'text' },
        ],
    };
    
    return [...commonHeaders, ...(streamHeaders[stream] || [])];
}

function formatCellValue(value, type) {
    if (value === null || value === undefined) {
        return '<span class="text-gray-400">-</span>';
    }
    
    switch (type) {
        case 'datetime':
            return formatTimestamp(value);
        case 'number':
            return typeof value === 'number' ? value.toFixed(2) : value;
        default:
            return value;
    }
}

function formatTimestamp(timestamp) {
    try {
        const date = new Date(timestamp);
        return date.toLocaleString();
    } catch {
        return timestamp;
    }
}

function getStreamIcon(stream) {
    const icons = {
        location: '📍',
        engines: '⚙️',
        fuel: '⛽',
        generators: '🔌',
        cctv: '📹',
        impact: '📊'
    };
    return icons[stream] || '📊';
}