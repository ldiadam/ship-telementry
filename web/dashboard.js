// Dashboard JavaScript - Interactive Charts, Maps, and Data Visualization

// API Base URL
const API_BASE = 'http://localhost:31180s';

// Global variables
let selectedVesselId = null;
let map = null;
let vesselMarker = null;
let trackPolyline = null;
let charts = {};

// Initialize dashboard
document.addEventListener('DOMContentLoaded', function () {
    initializeDashboard();
    setupEventListeners();
    loadVessels();

    // Add test button to fuel chart section after DOM is loaded
    setTimeout(() => {
        addFuelTestButton();
    }, 100);
});

function initializeDashboard() {
    // Initialize map
    initializeMap();

    // Initialize charts
    initializeCharts();

    // Set up tab switching
    setupTabSwitching();
}

function setupEventListeners() {
    // Vessel selector
    document.getElementById('vesselSelector').addEventListener('change', handleVesselChange);

    // Map controls
    document.getElementById('showTrack').addEventListener('click', showVesselTrack);
    document.getElementById('centerMap').addEventListener('click', centerMapOnVessel);
}

function setupTabSwitching() {
    const navLinks = document.querySelectorAll('.nav-link');
    const tabContents = document.querySelectorAll('.tab-content');

    navLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();

            // Remove active class from all links and tabs
            navLinks.forEach(l => l.classList.remove('active'));
            tabContents.forEach(t => t.classList.add('hidden'));

            // Add active class to clicked link
            link.classList.add('active');

            // Show corresponding tab
            const tabId = link.dataset.tab + '-tab';
            document.getElementById(tabId).classList.remove('hidden');

            // Fix map and charts when tab becomes visible
            setTimeout(() => {
                // Fix map size when location tab is shown
                if (link.dataset.tab === 'location' && map) {
                    map.invalidateSize();
                    if (vesselMarker) {
                        map.setView(vesselMarker.getLatLng(), 12);
                    }
                }

                // Resize charts when tab becomes visible
                Object.values(charts).forEach(chart => {
                    if (chart && chart.resize) chart.resize();
                });
            }, 100);
        });
    });
}

function initializeMap() {
    // Initialize Leaflet map with a slight delay to ensure container is ready
    setTimeout(() => {
        map = L.map('map').setView([34.0522, -118.2437], 8); // Default to Los Angeles with zoom level 8

        // Add OpenStreetMap tiles
        L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '© OpenStreetMap contributors'
        }).addTo(map);

        // Add scale control
        L.control.scale().addTo(map);

        // Force map to resize after initialization
        setTimeout(() => {
            if (map) {
                map.invalidateSize();
            }
        }, 100);
    }, 100);
}

async function loadVessels() {
    try {
        const response = await fetch(`${API_BASE}/vessels`);
        const vessels = await response.json();

        const selector = document.getElementById('vesselSelector');
        selector.innerHTML = '<option value="">Select Vessel</option>';

        vessels.forEach(vessel => {
            const option = document.createElement('option');
            option.value = vessel.id;
            option.textContent = `${vessel.name} ${vessel.imo ? '(IMO: ' + vessel.imo + ')' : ''}`;
            selector.appendChild(option);
        });

        // Auto-select first vessel if available
        if (vessels.length > 0) {
            selector.value = vessels[0].id;
            handleVesselChange();
        }

    } catch (error) {
        console.error('Error loading vessels:', error);
        showError('Failed to load vessels');
    }
}

async function handleVesselChange() {
    const vesselId = document.getElementById('vesselSelector').value;
    if (!vesselId) return;

    selectedVesselId = vesselId;

    // Load all vessel data
    await Promise.all([
        loadVesselMetrics(),
        loadLocationData(),
        loadPerformanceData(),
        loadEngineStatus(),
        loadSystemStatus()
    ]);
}

async function loadVesselMetrics() {
    if (!selectedVesselId) return;

    try {
        // Load latest data from different streams
        const [locationData, engineData, fuelData] = await Promise.all([
            fetchLatestData('location'),
            fetchLatestData('engines'),
            fetchLatestData('fuel')
        ]);

        // Update metric cards
        updateMetricCard('currentSpeed', locationData?.speed_knots || '--', 'knots');
        updateMetricCard('engineRPM', engineData?.rpm || '--', '');
        updateMetricCard('fuelLevel', fuelData?.level_percent || '--', '%');

        // Count online systems
        const systemsCount = await countOnlineSystems();
        updateMetricCard('systemsOnline', systemsCount.online + '/' + systemsCount.total, '');

    } catch (error) {
        console.error('Error loading vessel metrics:', error);
    }
}

async function fetchLatestData(stream) {
    try {
        const response = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=${stream}&limit=1`);
        if (!response.ok) {
            console.warn(`Failed to fetch ${stream} data: ${response.status}`);
            return null;
        }
        const data = await response.json();
        return data.items && data.items.length > 0 ? data.items[0] : null;
    } catch (error) {
        console.error(`Error fetching ${stream} data:`, error);
        return null;
    }
}

function updateMetricCard(elementId, value, unit) {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = value + (unit ? ' ' + unit : '');
    }
}

async function loadLocationData() {
    if (!selectedVesselId) return;

    try {
        console.log('🔍 Loading location data for vessel ID:', selectedVesselId);

        // Get location data - increase limit to get more records and ensure we get the latest
        const response = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=location&limit=200`);
        const data = await response.json();

        console.log('📊 API Response:', {
            totalItems: data.items?.length || 0,
            url: `${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=location&limit=200`
        });

        if (data.items && data.items.length > 0) {
            console.log('📋 Raw API data (ALL items with timestamps):', data.items.map((item, index) => ({
                index: index,
                id: item.id,
                ts: item.ts,
                lat: item.latitude,
                lon: item.longitude,
                speed: item.speed_knots
            })));

            // Sort by timestamp DESC to ensure latest is first (most recent timestamp)
            const sortedItems = data.items.sort((a, b) => new Date(b.ts) - new Date(a.ts));

            console.log('🔄 After sorting (first 10 items):', sortedItems.slice(0, 10).map(item => ({
                id: item.id,
                ts: item.ts,
                parsedDate: new Date(item.ts).toISOString(),
                lat: item.latitude,
                lon: item.longitude,
                speed: item.speed_knots
            })));

            // Get the actual latest location (most recent timestamp)
            const latest = sortedItems[0];

            console.log('⭐ Selected as LATEST (should match your DB record id=102):', {
                id: latest.id,
                ts: latest.ts,
                parsedDate: new Date(latest.ts).toISOString(),
                lat: latest.latitude,
                lon: latest.longitude,
                speed: latest.speed_knots,
                status: latest.status
            });

            // Update current position display with the actual latest data
            updateCurrentPosition(latest);

            // Update map with latest position
            updateMapLocation(latest, sortedItems);

            // Update location history - show in chronological order (latest first)
            updateLocationHistory(sortedItems.slice(0, 10));
        }

    } catch (error) {
        console.error('Error loading location data:', error);
    }
}

function updateCurrentPosition(locationData) {
    document.getElementById('currentLat').textContent = locationData.latitude?.toFixed(6) || '--';
    document.getElementById('currentLon').textContent = locationData.longitude?.toFixed(6) || '--';
    document.getElementById('currentCourse').textContent = locationData.course_degrees ? locationData.course_degrees.toFixed(1) + '°' : '--';
    document.getElementById('currentSpeedDetail').textContent = locationData.speed_knots ? locationData.speed_knots.toFixed(1) + ' knots' : '--';
    document.getElementById('currentStatus').textContent = locationData.status || '--';
}

function updateMapLocation(latest, allLocations) {
    if (!latest.latitude || !latest.longitude) return;

    const lat = latest.latitude;
    const lng = latest.longitude;

    // Remove existing marker
    if (vesselMarker) {
        map.removeLayer(vesselMarker);
    }

    // Add new marker
    vesselMarker = L.marker([lat, lng])
        .addTo(map)
        .bindPopup(`
            <div class="p-2">
                <h4 class="font-semibold">Current Position</h4>
                <p><strong>Lat:</strong> ${lat.toFixed(6)}</p>
                <p><strong>Lng:</strong> ${lng.toFixed(6)}</p>
                <p><strong>Speed:</strong> ${latest.speed_knots?.toFixed(1) || '--'} knots</p>
                <p><strong>Course:</strong> ${latest.course_degrees?.toFixed(1) || '--'}°</p>
                <p><strong>Status:</strong> ${latest.status || '--'}</p>
                <p><strong>Time:</strong> ${new Date(latest.ts).toLocaleString()}</p>
            </div>
        `);

    // Center map on vessel
    map.setView([lat, lng], 12);
}

function updateLocationHistory(locations) {
    const container = document.getElementById('locationHistory');
    container.innerHTML = '';

    console.log('Location history data:', locations.map(l => ({ ts: l.ts, lat: l.latitude, lon: l.longitude })));

    locations.forEach((location, index) => {
        if (!location.latitude || !location.longitude) return;

        const item = document.createElement('div');
        // Highlight the first item (latest) with a different background
        const bgClass = index === 0 ? 'bg-blue-50 border-blue-200' : 'bg-gray-50 border-gray-200';
        item.className = `p-3 rounded-lg border ${bgClass}`;

        const timestamp = new Date(location.ts);
        const isToday = timestamp.toDateString() === new Date().toDateString();
        const timeDisplay = isToday
            ? timestamp.toLocaleTimeString()
            : timestamp.toLocaleString();

        item.innerHTML = `
            <div class="flex justify-between items-start">
                <div class="text-sm">
                    <div class="font-medium">${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)}</div>
                    <div class="text-gray-600">${location.speed_knots?.toFixed(1) || '--'} knots</div>
                    ${index === 0 ? '<div class="text-xs text-blue-600 font-medium">CURRENT</div>' : ''}
                </div>
                <div class="text-xs text-gray-500">
                    ${timeDisplay}
                </div>
            </div>
        `;
        container.appendChild(item);
    });
}

async function showVesselTrack() {
    if (!selectedVesselId) return;

    try {
        const response = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=location&limit=100`);
        const data = await response.json();

        if (data.items && data.items.length > 1) {
            // Remove existing track
            if (trackPolyline) {
                map.removeLayer(trackPolyline);
            }

            // Create track from location points
            const trackPoints = data.items
                .filter(item => item.latitude && item.longitude)
                .map(item => [item.latitude, item.longitude])
                .reverse(); // Reverse to show chronological order

            if (trackPoints.length > 1) {
                trackPolyline = L.polyline(trackPoints, {
                    color: '#3182ce',
                    weight: 3,
                    opacity: 0.7
                }).addTo(map);

                // Fit map to track bounds
                map.fitBounds(trackPolyline.getBounds(), { padding: [20, 20] });
            }
        }

    } catch (error) {
        console.error('Error loading vessel track:', error);
    }
}

function centerMapOnVessel() {
    if (vesselMarker) {
        map.setView(vesselMarker.getLatLng(), 12);
    }
}

function initializeCharts() {
    // Engine charts will be created dynamically based on available engines
    charts.engines = {};

    // Fuel Chart - Support multiple tanks
    const fuelCtx = document.getElementById('fuelChart').getContext('2d');
    charts.fuel = new Chart(fuelCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [] // Will be populated dynamically based on available tanks
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100,
                    title: { display: true, text: 'Fuel Level (%)' }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        usePointStyle: true,
                        boxWidth: 6
                    }
                },
                tooltip: {
                    callbacks: {
                        footer: function (tooltipItems) {
                            // Add volume information if available
                            const dataIndex = tooltipItems[0].dataIndex;
                            const datasetIndex = tooltipItems[0].datasetIndex;
                            const tankData = charts.fuel.data.datasets[datasetIndex]._tankData;

                            if (tankData && tankData[dataIndex] && tankData[dataIndex].volume) {
                                return `Volume: ${tankData[dataIndex].volume.toFixed(0)} liters`;
                            }
                            return '';
                        }
                    }
                }
            }
        }
    });

    // Generator charts will be created dynamically based on available generators
    charts.generators = {};

    // Temperature Chart
    const tempCtx = document.getElementById('temperatureChart').getContext('2d');
    charts.temperature = new Chart(tempCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Engine Temp',
                data: [],
                borderColor: '#ef4444',
                backgroundColor: 'rgba(239, 68, 68, 0.1)',
                tension: 0.4
            }, {
                label: 'Fuel Temp',
                data: [],
                borderColor: '#8b5cf6',
                backgroundColor: 'rgba(139, 92, 246, 0.1)',
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    title: { display: true, text: 'Temperature (°C)' }
                }
            }
        }
    });

    // Vibration Chart - Fixed to use simple line chart instead of time scatter
    const vibrationCtx = document.getElementById('vibrationChart').getContext('2d');
    charts.vibration = new Chart(vibrationCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Acceleration (g)',
                data: [],
                backgroundColor: 'rgba(168, 85, 247, 0.1)',
                borderColor: '#a855f7',
                tension: 0.4
            }, {
                label: 'Shock (g)',
                data: [],
                backgroundColor: 'rgba(239, 68, 68, 0.1)',
                borderColor: '#ef4444',
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    title: { display: true, text: 'Force (g)' }
                }
            }
        }
    });

    // System Health Chart
    const healthCtx = document.getElementById('systemHealthChart').getContext('2d');
    charts.systemHealth = new Chart(healthCtx, {
        type: 'doughnut',
        data: {
            labels: ['Online', 'Warning', 'Offline'],
            datasets: [{
                data: [0, 0, 0],
                backgroundColor: ['#10b981', '#f59e0b', '#ef4444'],
                borderWidth: 2,
                borderColor: '#ffffff'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom'
                }
            }
        }
    });
}

async function loadPerformanceData() {
    if (!selectedVesselId) return;

    try {
        // Load engine data
        const engineResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=engines&limit=24`);
        const engineData = await engineResponse.json();

        if (engineData.items) {
            updateEngineChart(engineData.items.reverse());
        }

        // Load fuel data for all tanks
        console.log('Fetching fuel data...');
        const fuelResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=fuel&limit=100`);
        const fuelData = await fuelResponse.json();
        console.log('Fuel data response:', fuelData);

        if (fuelData.items) {
            console.log('Fuel data items before processing:', fuelData.items);

            // For testing purposes, if there's no tank_no in the data, create some test data
            if (fuelData.items.length > 0 && fuelData.items.every(item => item.tank_no === undefined)) {
                console.log('Creating test data with multiple tanks');
                // Create a copy of the items to simulate multiple tanks
                const testItems = [];

                // Add original items as tank 1
                fuelData.items.forEach(item => {
                    const tank1Item = { ...item, tank_no: 1 };
                    testItems.push(tank1Item);
                });

                // Add modified items as tank 2 with slightly different values
                fuelData.items.forEach(item => {
                    const tank2Item = {
                        ...item,
                        tank_no: 2,
                        level_percent: (item.level_percent || 0) * 0.8 // 80% of tank 1 values
                    };
                    testItems.push(tank2Item);
                });

                console.log('Test items created:', testItems);
                // Process and update with test data
                const processedData = processFuelData(testItems);
                updateFuelChart(processedData);
            } else {
                // Process and group the data by timestamp to get the most recent 24 readings
                // while preserving data from different tanks
                const processedData = processFuelData(fuelData.items);
                updateFuelChart(processedData);
            }
        }

        // Load generator data - get more historical data for trend analysis
        const generatorResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=generators&limit=24`);
        const generatorData = await generatorResponse.json();

        if (generatorData.items) {
            updateGeneratorChart(generatorData.items.reverse()); // Reverse for chronological order
        }

        // Load impact data
        const impactResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=impact&limit=50`);
        const impactData = await impactResponse.json();

        if (impactData.items) {
            updateVibrationChart(impactData.items);
        }

    } catch (error) {
        console.error('Error loading performance data:', error);
    }
}

function updateEngineChart(data) {
    console.log('Creating dynamic engine charts with data:', data);

    // Group data by engine number
    const engines = {};
    data.forEach(item => {
        const engineNo = item.engine_no || 1;
        if (!engines[engineNo]) {
            engines[engineNo] = [];
        }

        // Extract data from extra_json
        const extraData = item.extra_json || {};

        engines[engineNo].push({
            rpm: item.rpm || 0,
            temp: item.temp_c || 0,
            oilPressure: item.oil_pressure_bar || 0,
            coolantTemp: extraData['Coolant Temp(C)'] ? parseFloat(extraData['Coolant Temp(C)']) : null,
            load: extraData['Load(%)'] ? parseFloat(extraData['Load(%)']) : null,
            runningHours: extraData['Running Hours'] ? parseFloat(extraData['Running Hours']) : null,
            status: extraData['Status'] || 'unknown',
            alarms: item.alarms || null,
            timestamp: item.ts,
            id: item.id
        });
    });

    console.log('Grouped engine data:', engines);

    // Clear existing engine charts
    clearEngineCharts();

    // Create individual chart for each engine
    Object.keys(engines).sort((a, b) => parseInt(a) - parseInt(b)).forEach(engineNo => {
        createEngineChart(engineNo, engines[engineNo]);
    });
}

function clearEngineCharts() {
    // Destroy existing engine charts
    Object.values(charts.engines).forEach(chart => {
        if (chart && chart.destroy) {
            chart.destroy();
        }
    });
    charts.engines = {};

    // Clear the container
    const container = document.getElementById('engineChartsContainer');
    if (container) {
        container.innerHTML = '';
    }
}

function createEngineChart(engineNo, engineData) {
    console.log(`Creating chart for Engine ${engineNo}:`, engineData);

    // Extract unique timestamps and sort chronologically
    const uniqueTimestamps = [...new Set(engineData.map(item => item.timestamp))].sort((a, b) => new Date(a) - new Date(b));
    const labels = uniqueTimestamps.map(ts => new Date(ts).toLocaleTimeString());

    // Prepare datasets for all available metrics
    const datasets = [];
    const colors = ['#3182ce', '#ef4444', '#10b981', '#f59e0b', '#8b5cf6', '#f97316'];
    let colorIndex = 0;

    // Helper function to extract data for a specific metric
    const extractMetricData = (metricKey, metricName, unit = '') => {
        const data = uniqueTimestamps.map(timestamp => {
            const dataPoint = engineData.find(item => item.timestamp === timestamp);
            return dataPoint && dataPoint[metricKey] !== null ? dataPoint[metricKey] : null;
        });

        // Only add dataset if we have some non-null data
        if (data.some(value => value !== null)) {
            datasets.push({
                label: `${metricName}${unit ? ' (' + unit + ')' : ''}`,
                data: data,
                borderColor: colors[colorIndex % colors.length],
                backgroundColor: `rgba(${hexToRgb(colors[colorIndex % colors.length])}, 0.1)`,
                tension: 0.4,
                fill: false,
                pointRadius: 3,
                pointHoverRadius: 5
            });
            colorIndex++;
        }
    };

    // Add datasets for all available metrics
    extractMetricData('rpm', 'RPM');
    extractMetricData('temp', 'Engine Temp', '°C');
    extractMetricData('oilPressure', 'Oil Pressure', 'bar');
    extractMetricData('coolantTemp', 'Coolant Temp', '°C');
    extractMetricData('load', 'Load', '%');

    // Get engine status for title
    const latestData = engineData[engineData.length - 1];
    const status = latestData ? latestData.status : 'unknown';
    const hasAlarms = latestData && latestData.alarms;
    const statusColor = status === 'running' ? 'text-green-600' : status === 'standby' ? 'text-blue-600' : 'text-gray-600';
    const alarmColor = hasAlarms ? 'text-red-600' : '';

    // Create chart container
    const container = document.getElementById('engineChartsContainer');
    const chartCard = document.createElement('div');
    chartCard.className = 'dashboard-card p-6 mb-6';
    chartCard.innerHTML = `
        <h3 class="text-lg font-semibold mb-4">
            <i class="fas fa-cogs mr-2 text-blue-600"></i>
            Engine ${engineNo} Performance 
            <span class="text-sm ${statusColor} ml-2">(${status})</span>
            ${hasAlarms ? `<span class="text-xs ${alarmColor} ml-2">⚠️ ${latestData.alarms}</span>` : ''}
        </h3>
        <div class="chart-container">
            <canvas id="engine${engineNo}Chart"></canvas>
        </div>
    `;
    container.appendChild(chartCard);

    // Create the chart
    const ctx = document.getElementById(`engine${engineNo}Chart`).getContext('2d');
    charts.engines[engineNo] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    title: { display: true, text: 'Values' }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        usePointStyle: true,
                        boxWidth: 6
                    }
                },
                tooltip: {
                    callbacks: {
                        footer: function (tooltipItems) {
                            const dataIndex = tooltipItems[0].dataIndex;
                            const dataPoint = engineData.find(item =>
                                item.timestamp === uniqueTimestamps[dataIndex]
                            );

                            if (dataPoint) {
                                let footer = '';
                                if (dataPoint.status) footer += `Status: ${dataPoint.status}\n`;
                                if (dataPoint.alarms) footer += `Alarms: ${dataPoint.alarms}\n`;
                                if (dataPoint.id) footer += `ID: ${dataPoint.id}`;
                                return footer;
                            }
                            return '';
                        }
                    }
                }
            }
        }
    });

    console.log(`Engine ${engineNo} chart created with ${datasets.length} datasets`);
}

function updateFuelChart(data) {
    console.log('Updating fuel chart with data:', data);
    console.log('Raw fuel data items:', data.map(item => ({
        id: item.id,
        tank_no: item.tank_no,
        ts: item.ts,
        level_percent: item.level_percent,
        volume_liters: item.volume_liters
    })));

    // Extract unique timestamps and sort them chronologically
    const uniqueTimestamps = [...new Set(data.map(item => item.ts))].sort((a, b) => new Date(a) - new Date(b));
    const labels = uniqueTimestamps.map(ts => new Date(ts).toLocaleTimeString());
    console.log('Unique timestamps:', uniqueTimestamps.length, 'Labels:', labels);

    // Group data by tank number
    const tanks = {};
    data.forEach(item => {
        const tankNo = item.tank_no || 1; // Default to tank 1 if not specified
        if (!tanks[tankNo]) {
            tanks[tankNo] = [];
        }
        tanks[tankNo].push({
            level: item.level_percent || 0,
            volume: item.volume_liters || 0,
            temp: item.temp_c || 0,
            timestamp: item.ts,
            id: item.id
        });
    });

    console.log('Grouped tanks data:', tanks);
    console.log('Number of tanks:', Object.keys(tanks).length);

    // Log detailed tank information
    Object.keys(tanks).forEach(tankNo => {
        console.log(`Tank ${tankNo} data points:`, tanks[tankNo].map(item => ({
            id: item.id,
            timestamp: item.timestamp,
            level: item.level,
            volume: item.volume
        })));
    });

    // Clear existing datasets
    charts.fuel.data.datasets = [];

    // Create a dataset for each tank with different colors
    const colors = ['#10b981', '#3182ce', '#8b5cf6', '#f59e0b', '#ef4444'];

    Object.keys(tanks).sort((a, b) => parseInt(a) - parseInt(b)).forEach((tankNo, index) => {
        // Create arrays for this tank's data points
        const tankData = [];
        const tankVolumes = [];

        // For each timestamp, find the corresponding data point for this tank
        uniqueTimestamps.forEach(timestamp => {
            // Find the data point for this tank at this timestamp
            const tankItems = tanks[tankNo];
            const dataPoint = tankItems.find(item => item.timestamp === timestamp);

            if (dataPoint) {
                tankData.push(dataPoint.level);
                tankVolumes.push({
                    volume: dataPoint.volume,
                    temp: dataPoint.temp,
                    id: dataPoint.id
                });
            } else {
                // If no data point for this timestamp, use null
                tankData.push(null);
                tankVolumes.push(null);
            }
        });

        // Add dataset for this tank
        const dataset = {
            label: `Tank ${tankNo} Level (%)`,
            data: tankData,
            borderColor: colors[index % colors.length],
            backgroundColor: `rgba(${hexToRgb(colors[index % colors.length])}, 0.1)`,
            tension: 0.4,
            fill: false,
            pointRadius: 4,
            pointHoverRadius: 6,
            _tankData: tankVolumes // Store volume data for tooltips
        };

        console.log(`Adding dataset for Tank ${tankNo}:`, {
            label: dataset.label,
            dataPoints: dataset.data,
            color: dataset.borderColor
        });
        charts.fuel.data.datasets.push(dataset);
    });

    charts.fuel.data.labels = labels;
    console.log('Final fuel chart configuration:');
    console.log('- Labels:', charts.fuel.data.labels);
    console.log('- Datasets:', charts.fuel.data.datasets.map(ds => ({
        label: ds.label,
        data: ds.data,
        color: ds.borderColor
    })));

    charts.fuel.update();

    // Update the chart title to show tank information and data summary
    const tankCount = Object.keys(tanks).length;
    const totalDataPoints = data.length;
    const chartTitle = document.querySelector('#fuelChart').closest('.dashboard-card').querySelector('h3');
    if (chartTitle) {
        chartTitle.innerHTML = `
            <i class="fas fa-gas-pump mr-2 text-green-600"></i>
            Fuel Levels - ${tankCount} Tank${tankCount !== 1 ? 's' : ''} (${totalDataPoints} readings)
        `;
        console.log(`Updated chart title: ${tankCount} tanks, ${totalDataPoints} readings`);
    }

    // Log tank levels for verification
    Object.keys(tanks).forEach(tankNo => {
        const latestReading = tanks[tankNo][tanks[tankNo].length - 1];
        if (latestReading) {
            console.log(`Tank ${tankNo} latest: ${latestReading.level}% (${latestReading.volume}L)`);
        }
    });
}

// Helper function to convert hex color to RGB for transparency
function hexToRgb(hex) {
    // Remove # if present
    hex = hex.replace('#', '');

    // Parse the hex values
    const r = parseInt(hex.substring(0, 2), 16);
    const g = parseInt(hex.substring(2, 4), 16);
    const b = parseInt(hex.substring(4, 6), 16);

    return `${r}, ${g}, ${b}`;
}

// Process fuel data to handle multiple tanks and ensure we get a good time series
function processFuelData(items) {
    console.log('Processing fuel data, received items:', items);

    // Check if we have tank_no in the data
    const hasTankNumbers = items.some(item => item.tank_no !== undefined);
    console.log('Data has tank numbers:', hasTankNumbers);

    // If no tank numbers are present, add them artificially for testing
    if (!hasTankNumbers && items.length > 0) {
        console.log('No tank numbers found, adding test tank numbers');
        // Split the items into two artificial tanks for testing
        const halfLength = Math.floor(items.length / 2);
        for (let i = 0; i < items.length; i++) {
            items[i].tank_no = i < halfLength ? 1 : 2;
        }
    }

    // First, sort by timestamp (newest first)
    const sortedItems = [...items].sort((a, b) => new Date(b.ts) - new Date(a.ts));

    // Group by tank number
    const tankGroups = {};
    sortedItems.forEach(item => {
        const tankNo = item.tank_no || 1;
        if (!tankGroups[tankNo]) {
            tankGroups[tankNo] = [];
        }
        tankGroups[tankNo].push(item);
    });

    console.log('Tank groups after processing:', tankGroups);

    // Get unique timestamps from all tanks, limited to 24 most recent
    const allTimestamps = [...new Set(sortedItems.map(item => item.ts))]
        .sort((a, b) => new Date(b) - new Date(a))
        .slice(0, 24)
        .reverse(); // Reverse to get chronological order

    console.log('Unique timestamps (24 most recent):', allTimestamps);

    // For each timestamp, include data from all tanks
    const result = [];

    // For each tank, add its readings for each timestamp
    Object.keys(tankGroups).forEach(tankNo => {
        const tankReadings = tankGroups[tankNo];

        allTimestamps.forEach(timestamp => {
            // Find the closest reading for this tank to the current timestamp
            const closestReading = tankReadings.find(reading => reading.ts === timestamp);

            if (closestReading) {
                // Make sure we have a tank_no property
                if (closestReading.tank_no === undefined) {
                    closestReading.tank_no = parseInt(tankNo);
                }
                result.push(closestReading);
            }
        });
    });

    console.log('Final processed result:', result);
    console.log('Number of tanks in result:', new Set(result.map(item => item.tank_no)).size);
    return result;
}

function updateGeneratorChart(data) {
    console.log('Creating dynamic generator charts with data:', data);

    // Group data by generator number
    const generators = {};
    data.forEach(item => {
        const genNo = item.gen_no || 1;
        if (!generators[genNo]) {
            generators[genNo] = [];
        }

        // Extract data from extra_json
        const extraData = item.extra_json || {};

        generators[genNo].push({
            load: item.load_kw,
            voltage: item.voltage_v,
            frequency: item.frequency_hz,
            coolantTemp: extraData['Coolant Temp(C)'] ? parseFloat(extraData['Coolant Temp(C)']) : null,
            oilPressure: extraData['Oil Pressure(bar)'] ? parseFloat(extraData['Oil Pressure(bar)']) : null,
            output: extraData['Output(kW)'] ? parseFloat(extraData['Output(kW)']) : null,
            runningHours: extraData['Running Hours'] ? parseFloat(extraData['Running Hours']) : null,
            status: extraData['Status'] || 'unknown',
            notes: extraData['Notes'] || null,
            timestamp: item.ts,
            id: item.id
        });
    });

    console.log('Grouped generator data:', generators);

    // Clear existing generator charts
    clearGeneratorCharts();

    // Create individual chart for each generator
    Object.keys(generators).sort((a, b) => parseInt(a) - parseInt(b)).forEach(genNo => {
        createGeneratorChart(genNo, generators[genNo]);
    });
}

function clearGeneratorCharts() {
    // Destroy existing generator charts
    Object.values(charts.generators).forEach(chart => {
        if (chart && chart.destroy) {
            chart.destroy();
        }
    });
    charts.generators = {};

    // Clear the container
    const container = document.getElementById('generatorChartsContainer');
    if (container) {
        container.innerHTML = '';
    }
}

function createGeneratorChart(genNo, generatorData) {
    console.log(`Creating chart for Generator ${genNo}:`, generatorData);

    // Extract unique timestamps and sort chronologically
    const uniqueTimestamps = [...new Set(generatorData.map(item => item.timestamp))].sort((a, b) => new Date(a) - new Date(b));
    const labels = uniqueTimestamps.map(ts => new Date(ts).toLocaleTimeString());

    // Prepare datasets for all available metrics
    const datasets = [];
    const colors = ['#f59e0b', '#ef4444', '#10b981', '#3182ce', '#8b5cf6', '#f97316'];
    let colorIndex = 0;

    // Helper function to extract data for a specific metric
    const extractMetricData = (metricKey, metricName, unit = '') => {
        const data = uniqueTimestamps.map(timestamp => {
            const dataPoint = generatorData.find(item => item.timestamp === timestamp);
            return dataPoint && dataPoint[metricKey] !== null ? dataPoint[metricKey] : null;
        });

        // Only add dataset if we have some non-null data
        if (data.some(value => value !== null)) {
            datasets.push({
                label: `${metricName}${unit ? ' (' + unit + ')' : ''}`,
                data: data,
                borderColor: colors[colorIndex % colors.length],
                backgroundColor: `rgba(${hexToRgb(colors[colorIndex % colors.length])}, 0.1)`,
                tension: 0.4,
                fill: false,
                pointRadius: 3,
                pointHoverRadius: 5
            });
            colorIndex++;
        }
    };

    // Add datasets for all available metrics
    extractMetricData('load', 'Load', 'kW');
    extractMetricData('voltage', 'Voltage', 'V');
    extractMetricData('frequency', 'Frequency', 'Hz');
    extractMetricData('coolantTemp', 'Coolant Temp', '°C');
    extractMetricData('oilPressure', 'Oil Pressure', 'bar');
    extractMetricData('output', 'Output', 'kW');

    // Get generator status for title
    const latestData = generatorData[generatorData.length - 1];
    const status = latestData ? latestData.status : 'unknown';
    const statusColor = status === 'running' ? 'text-green-600' : status === 'tested' ? 'text-blue-600' : 'text-gray-600';

    // Create chart container
    const container = document.getElementById('generatorChartsContainer');
    const chartCard = document.createElement('div');
    chartCard.className = 'dashboard-card p-6 mb-6';
    chartCard.innerHTML = `
        <h3 class="text-lg font-semibold mb-4">
            <i class="fas fa-bolt mr-2 text-yellow-600"></i>
            Generator ${genNo} Performance 
            <span class="text-sm ${statusColor} ml-2">(${status})</span>
        </h3>
        <div class="chart-container">
            <canvas id="generator${genNo}Chart"></canvas>
        </div>
    `;
    container.appendChild(chartCard);

    // Create the chart
    const ctx = document.getElementById(`generator${genNo}Chart`).getContext('2d');
    charts.generators[genNo] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    title: { display: true, text: 'Values' }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        usePointStyle: true,
                        boxWidth: 6
                    }
                },
                tooltip: {
                    callbacks: {
                        footer: function (tooltipItems) {
                            const dataIndex = tooltipItems[0].dataIndex;
                            const dataPoint = generatorData.find(item =>
                                item.timestamp === uniqueTimestamps[dataIndex]
                            );

                            if (dataPoint) {
                                let footer = '';
                                if (dataPoint.status) footer += `Status: ${dataPoint.status}\n`;
                                if (dataPoint.notes) footer += `Notes: ${dataPoint.notes}\n`;
                                if (dataPoint.id) footer += `ID: ${dataPoint.id}`;
                                return footer;
                            }
                            return '';
                        }
                    }
                }
            }
        }
    });

    console.log(`Generator ${genNo} chart created with ${datasets.length} datasets`);
}

function updateVibrationChart(data) {
    const labels = data.map(item => new Date(item.ts).toLocaleTimeString());
    const accelData = data.map(item => item.accel_g || 0);
    const shockData = data.map(item => item.shock_g || 0);

    charts.vibration.data.labels = labels;
    charts.vibration.data.datasets[0].data = accelData;
    charts.vibration.data.datasets[1].data = shockData;
    charts.vibration.update();
}

async function loadEngineStatus() {
    if (!selectedVesselId) return;

    try {
        // Load latest engine data to show current status
        const engineResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=engines&limit=10`);
        const engineData = await engineResponse.json();

        if (engineData.items) {
            updateEngineStatus(engineData.items);
        }

    } catch (error) {
        console.error('Error loading engine status:', error);
    }
}

function updateEngineStatus(data) {
    const container = document.getElementById('engineStatus');
    container.innerHTML = '';

    // Group engines by engine_no to get latest status for each
    const engines = {};
    data.forEach(item => {
        const engineNo = item.engine_no || 1;
        if (!engines[engineNo] || new Date(item.ts) > new Date(engines[engineNo].ts)) {
            engines[engineNo] = item;
        }
    });

    // Display each engine's status
    Object.keys(engines).sort().forEach(engineNo => {
        const engine = engines[engineNo];

        // Determine engine status based on RPM and alarms
        let status = 'offline';
        let statusColor = 'status-offline';

        if (engine.rpm && engine.rpm > 0) {
            if (engine.alarms && engine.alarms.toLowerCase() !== 'ok' && engine.alarms.toLowerCase() !== '') {
                status = 'warning';
                statusColor = 'status-warning';
            } else {
                status = 'running';
                statusColor = 'status-online';
            }
        }

        const engineItem = document.createElement('div');
        engineItem.className = 'flex items-center justify-between p-4 bg-gray-50 rounded-lg border border-gray-200';
        engineItem.innerHTML = `
            <div class="flex items-center">
                <div class="bg-blue-100 p-3 rounded-full mr-4">
                    <i class="fas fa-cog text-blue-600 text-xl"></i>
                </div>
                <div>
                    <div class="font-semibold text-lg">Engine ${engineNo}</div>
                    <div class="flex items-center mt-1">
                        <span class="status-indicator ${statusColor}"></span>
                        <span class="text-sm font-medium capitalize">${status}</span>
                    </div>
                    <div class="text-xs text-gray-500 mt-1">
                        Last update: ${new Date(engine.ts).toLocaleTimeString()}
                    </div>
                </div>
            </div>
            <div class="text-right">
                <div class="space-y-1">
                    <div class="text-sm">
                        <span class="font-medium">RPM:</span> 
                        <span class="text-blue-600">${engine.rpm ? engine.rpm.toFixed(0) : '--'}</span>
                    </div>
                    <div class="text-sm">
                        <span class="font-medium">Temp:</span> 
                        <span class="text-red-600">${engine.temp_c ? engine.temp_c.toFixed(1) + '°C' : '--'}</span>
                    </div>
                    <div class="text-sm">
                        <span class="font-medium">Oil Pressure:</span> 
                        <span class="text-green-600">${engine.oil_pressure_bar ? engine.oil_pressure_bar.toFixed(1) + ' bar' : '--'}</span>
                    </div>
                    ${engine.alarms && engine.alarms !== 'OK' ? `
                    <div class="text-xs text-orange-600 font-medium">
                        ⚠️ ${engine.alarms}
                    </div>
                    ` : ''}
                </div>
            </div>
        `;
        container.appendChild(engineItem);
    });

    // If no engines found, show message
    if (Object.keys(engines).length === 0) {
        container.innerHTML = `
            <div class="col-span-full text-center py-8 text-gray-500">
                <i class="fas fa-cog text-4xl mb-2"></i>
                <p>No engine data available</p>
            </div>
        `;
    }
}

async function loadSystemStatus() {
    if (!selectedVesselId) return;

    try {
        // Load CCTV status
        const cctvResponse = await fetch(`${API_BASE}/vessels/${selectedVesselId}/telemetry?stream=cctv&limit=10`);
        const cctvData = await cctvResponse.json();

        if (cctvData.items) {
            updateCCTVStatus(cctvData.items);
        }

        // Update system status overview
        updateSystemStatusOverview();

        // Update system health chart
        updateSystemHealthChart();

    } catch (error) {
        console.error('Error loading system status:', error);
    }
}

function updateCCTVStatus(data) {
    const container = document.getElementById('cctvStatus');
    container.innerHTML = '';

    data.forEach(item => {
        const statusClass = getStatusClass(item.status);
        const statusColor = getStatusColor(item.status);

        const statusItem = document.createElement('div');
        statusItem.className = 'flex items-center justify-between p-3 bg-gray-50 rounded-lg';
        statusItem.innerHTML = `
            <div class="flex items-center">
                <span class="status-indicator ${statusColor}"></span>
                <div>
                    <div class="font-medium">${item.cam_id || 'Unknown Camera'}</div>
                    <div class="text-sm text-gray-600">${item.status || 'Unknown'}</div>
                </div>
            </div>
            <div class="text-right text-sm">
                <div class="font-medium">${item.uptime_percent ? item.uptime_percent.toFixed(1) + '%' : '--'}</div>
                <div class="text-gray-600">Uptime</div>
            </div>
        `;
        container.appendChild(statusItem);
    });
}

function updateSystemStatusOverview() {
    const container = document.getElementById('systemStatus');
    container.innerHTML = '';

    const systems = [
        { name: 'Main Engine', status: 'online', icon: 'fas fa-cog' },
        { name: 'Navigation', status: 'online', icon: 'fas fa-compass' },
        { name: 'Communication', status: 'warning', icon: 'fas fa-radio' },
        { name: 'Power Generation', status: 'online', icon: 'fas fa-bolt' },
        { name: 'Fuel System', status: 'online', icon: 'fas fa-gas-pump' },
        { name: 'CCTV Network', status: 'warning', icon: 'fas fa-video' }
    ];

    systems.forEach(system => {
        const statusColor = getStatusColor(system.status);

        const systemItem = document.createElement('div');
        systemItem.className = 'flex items-center p-3 bg-gray-50 rounded-lg';
        systemItem.innerHTML = `
            <i class="${system.icon} text-2xl text-blue-600 mr-3"></i>
            <div class="flex-1">
                <div class="font-medium">${system.name}</div>
                <div class="flex items-center mt-1">
                    <span class="status-indicator ${statusColor}"></span>
                    <span class="text-sm capitalize">${system.status}</span>
                </div>
            </div>
        `;
        container.appendChild(systemItem);
    });
}

function updateSystemHealthChart() {
    const onlineCount = 4;
    const warningCount = 2;
    const offlineCount = 0;

    charts.systemHealth.data.datasets[0].data = [onlineCount, warningCount, offlineCount];
    charts.systemHealth.update();
}

async function countOnlineSystems() {
    // This would normally query multiple endpoints
    // For demo purposes, return mock data
    return { online: 4, total: 6 };
}

function getStatusClass(status) {
    switch (status?.toLowerCase()) {
        case 'online':
        case 'ok':
        case 'active':
            return 'status-online';
        case 'warning':
        case 'degraded':
            return 'status-warning';
        case 'offline':
        case 'error':
        case 'failed':
            return 'status-offline';
        default:
            return 'status-warning';
    }
}

function getStatusColor(status) {
    switch (status?.toLowerCase()) {
        case 'online':
        case 'ok':
        case 'active':
            return 'status-online';
        case 'warning':
        case 'degraded':
            return 'status-warning';
        case 'offline':
        case 'error':
        case 'failed':
            return 'status-offline';
        default:
            return 'status-warning';
    }
}

function showError(message) {
    console.error(message);
    // You could add a toast notification here
}

// Auto-refresh data every 30 seconds
setInterval(() => {
    if (selectedVesselId) {
        loadVesselMetrics();
        loadLocationData();
    }
}, 30000);
//

function addFuelTestButton() {
    // Find the fuel chart card header
    const fuelChartCard = document.querySelector('#fuelChart').closest('.dashboard-card');
    const fuelChartHeader = fuelChartCard.querySelector('h3');

    if (fuelChartHeader && !document.getElementById('fuelTestButton')) {
        // Create a container for the header content
        const headerContainer = document.createElement('div');
        headerContainer.className = 'flex items-center justify-between';

        // Move existing header content to the left side
        const headerContent = document.createElement('div');
        headerContent.innerHTML = fuelChartHeader.innerHTML;

        // Create the test button
        const testButton = document.createElement('button');
        testButton.id = 'fuelTestButton';
        testButton.textContent = 'Test Data';
        testButton.className = 'px-3 py-1 text-xs bg-blue-100 text-blue-700 rounded-md hover:bg-blue-200 transition-colors';
        testButton.onclick = function () {
            // Create test data with multiple tanks
            const testData = [];
            const now = new Date();

            // Create 24 data points for 2 tanks with very distinct patterns
            for (let i = 0; i < 24; i++) {
                const timestamp = new Date(now.getTime() - (23 - i) * 3600000).toISOString();

                // Tank 1 data - decreasing trend
                testData.push({
                    tank_no: 1,
                    ts: timestamp,
                    level_percent: 90 - i * 1.5,
                    volume_liters: 9000 - i * 150,
                    temp_c: 25 + Math.random() * 5
                });

                // Tank 2 data - increasing then decreasing pattern
                testData.push({
                    tank_no: 2,
                    ts: timestamp,
                    level_percent: i < 12 ? 40 + i * 2 : 64 - (i - 12) * 3,
                    volume_liters: i < 12 ? 4000 + i * 200 : 6400 - (i - 12) * 300,
                    temp_c: 22 + Math.random() * 4
                });
            }

            console.log('Created test data for multiple tanks:', testData);
            console.log('Test data tank 1 count:', testData.filter(item => item.tank_no === 1).length);
            console.log('Test data tank 2 count:', testData.filter(item => item.tank_no === 2).length);

            // Update the chart directly with test data
            updateFuelChart(testData);
        };

        // Assemble the header
        headerContainer.appendChild(headerContent);
        headerContainer.appendChild(testButton);

        // Replace the original header content
        fuelChartHeader.innerHTML = '';
        fuelChartHeader.appendChild(headerContainer);

        console.log('Added fuel test button to chart header');
    }
}