// API Base URL
const API_BASE = '/v1';

// Global state
let storageData = [];
let openAPIData = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    loadDashboard();
    
    // Add event listeners for tab changes
    document.getElementById('storage-tab').addEventListener('shown.bs.tab', function() {
        loadStorage();
    });
    
    document.getElementById('openapi-tab').addEventListener('shown.bs.tab', function() {
        if (!openAPIData) {
            loadOpenAPI();
        }
    });
});

// Refresh all data
function refreshAll() {
    const activeTab = document.querySelector('.nav-link.active').id;
    if (activeTab === 'dashboard-tab') {
        loadDashboard();
    } else if (activeTab === 'storage-tab') {
        loadStorage();
    } else if (activeTab === 'openapi-tab') {
        loadOpenAPI();
    }
}

// Load dashboard data
async function loadDashboard() {
    try {
        const [health, storage] = await Promise.all([
            fetch(`${API_BASE}/sys/health`).then(r => r.json()),
            fetch(`${API_BASE}/sys/storage`).then(r => r.json())
        ]);
        
        renderPluginStatus(health);
        renderQuickStats(health, storage);
        
        // Try to load OpenAPI for endpoints
        try {
            const openapi = await fetch(`${API_BASE}/sys/plugins/catalog/openapi`).then(r => r.json());
            renderEndpoints(openapi);
        } catch (e) {
            document.getElementById('endpointsList').innerHTML = '<p class="text-muted">OpenAPI specification not available</p>';
        }
    } catch (error) {
        console.error('Error loading dashboard:', error);
        document.getElementById('pluginStatus').innerHTML = `<div class="alert alert-danger">Error: ${error.message}</div>`;
    }
}

// Render plugin status
function renderPluginStatus(health) {
    const statusHtml = `
        <div class="mb-2">
            <strong>Status:</strong>
            <span class="badge ${health.plugin_running ? 'bg-success' : 'bg-danger'} status-badge">
                ${health.plugin_running ? '✓ Running' : '✗ Stopped'}
            </span>
        </div>
        ${health.plugin_type ? `<div><strong>Type:</strong> ${health.plugin_type}</div>` : ''}
        ${health.mount_path ? `<div><strong>Mount Path:</strong> ${health.mount_path}</div>` : ''}
        ${health.plugin_name ? `<div><strong>Plugin Name:</strong> ${health.plugin_name}</div>` : ''}
        ${health.plugin_version ? `<div><strong>Version:</strong> ${health.plugin_version}</div>` : ''}
    `;
    document.getElementById('pluginStatus').innerHTML = statusHtml;
}

// Render quick stats
function renderQuickStats(health, storage) {
    const statsHtml = `
        <div class="row text-center">
            <div class="col-6">
                <h3 class="text-primary">${storage.length}</h3>
                <small class="text-muted">Storage Keys</small>
            </div>
            <div class="col-6">
                <h3 class="text-info">${health.storage_entries || 0}</h3>
                <small class="text-muted">Storage Entries</small>
            </div>
        </div>
    `;
    document.getElementById('quickStats').innerHTML = statsHtml;
}

// Render endpoints
function renderEndpoints(openapi) {
    if (!openapi || !openapi.paths) {
        document.getElementById('endpointsList').innerHTML = '<p class="text-muted">No endpoints found</p>';
        return;
    }
    
    let html = '<div class="table-responsive"><table class="table table-hover">';
    html += '<thead><tr><th>Path</th><th>Methods</th><th>Description</th></tr></thead><tbody>';
    
    for (const [path, pathData] of Object.entries(openapi.paths)) {
        const methods = Object.keys(pathData).map(method => {
            const cls = `method-${method.toLowerCase()}`;
            return `<span class="badge ${cls} method-badge me-1">${method.toUpperCase()}</span>`;
        }).join('');
        
        const firstMethod = pathData[Object.keys(pathData)[0]];
        const description = firstMethod?.summary || firstMethod?.description || '';
        
        html += `<tr>
            <td><code class="endpoint-path">${path}</code></td>
            <td>${methods}</td>
            <td><small>${description}</small></td>
        </tr>`;
    }
    
    html += '</tbody></table></div>';
    document.getElementById('endpointsList').innerHTML = html;
}

// Load OpenAPI specification
async function loadOpenAPI() {
    const content = document.getElementById('openapiContent');
    content.innerHTML = '<div class="spinner-border spinner-border-sm" role="status"></div> Loading...';
    
    try {
        const data = await fetch(`${API_BASE}/sys/plugins/catalog/openapi`).then(r => r.json());
        openAPIData = data;
        
        let html = '';
        
        if (data.info) {
            html += '<div class="mb-4">';
            html += `<h5>${data.info.title || 'API Documentation'}</h5>`;
            if (data.info.version) html += `<p><strong>Version:</strong> ${data.info.version}</p>`;
            if (data.info.description) html += `<p>${data.info.description}</p>`;
            html += '</div>';
        }
        
        if (data.paths) {
            html += '<div class="accordion" id="openapiAccordion">';
            let index = 0;
            
            for (const [path, pathData] of Object.entries(data.paths)) {
                html += `<div class="accordion-item">
                    <h2 class="accordion-header" id="heading${index}">
                        <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#collapse${index}">
                            <code class="endpoint-path me-2">${path}</code>
                            ${Object.keys(pathData).map(m => `<span class="badge method-${m.toLowerCase()} method-badge me-1">${m.toUpperCase()}</span>`).join('')}
                        </button>
                    </h2>
                    <div id="collapse${index}" class="accordion-collapse collapse" data-bs-parent="#openapiAccordion">
                        <div class="accordion-body">`;
                
                for (const [method, operation] of Object.entries(pathData)) {
                    html += `<div class="mb-3">
                        <h6><span class="badge method-${method.toLowerCase()} method-badge">${method.toUpperCase()}</span></h6>`;
                    
                    if (operation.summary) html += `<p><strong>Summary:</strong> ${operation.summary}</p>`;
                    if (operation.description) html += `<p>${operation.description}</p>`;
                    
                    if (operation.parameters) {
                        html += '<p><strong>Parameters:</strong></p><ul>';
                        operation.parameters.forEach(param => {
                            html += `<li><code>${param.name}</code> (${param.in}) ${param.required ? '<span class="badge bg-warning text-dark">required</span>' : ''} - ${param.description || ''}</li>`;
                        });
                        html += '</ul>';
                    }
                    
                    html += '</div>';
                }
                
                html += '</div></div></div>';
                index++;
            }
            
            html += '</div>';
        }
        
        html += '<div class="mt-4"><h6>Raw JSON:</h6><button class="btn btn-sm btn-secondary mb-2" onclick="copyToClipboard(JSON.stringify(openAPIData, null, 2))">Copy to Clipboard</button>';
        html += `<pre>${JSON.stringify(data, null, 2)}</pre></div>`;
        
        content.innerHTML = html;
    } catch (error) {
        content.innerHTML = `<div class="alert alert-danger">Error loading OpenAPI: ${error.message}</div>`;
    }
}

// Load storage data
async function loadStorage() {
    const content = document.getElementById('storageContent');
    content.innerHTML = '<div class="spinner-border spinner-border-sm" role="status"></div> Loading...';
    
    try {
        storageData = await fetch(`${API_BASE}/sys/storage`).then(r => r.json());
        renderStorage(storageData);
    } catch (error) {
        content.innerHTML = `<div class="alert alert-danger">Error loading storage: ${error.message}</div>`;
    }
}

// Render storage items
function renderStorage(data) {
    const content = document.getElementById('storageContent');
    
    if (!data || data.length === 0) {
        content.innerHTML = '<p class="text-muted">No storage keys found</p>';
        return;
    }
    
    let html = '<div class="accordion" id="storageAccordion">';
    
    data.forEach((item, index) => {
        const size = new Blob([item.value]).size;
        const sizeStr = size < 1024 ? `${size} B` : `${(size / 1024).toFixed(1)} KB`;
        
        html += `<div class="accordion-item storage-key">
            <h2 class="accordion-header" id="storageHeading${index}">
                <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#storageCollapse${index}">
                    <div class="d-flex justify-content-between w-100 me-3">
                        <code>${item.key}</code>
                        <small class="text-muted">${sizeStr}</small>
                    </div>
                </button>
            </h2>
            <div id="storageCollapse${index}" class="accordion-collapse collapse" data-bs-parent="#storageAccordion">
                <div class="accordion-body">
                    <button class="btn btn-sm btn-secondary mb-2" onclick="copyToClipboard('${item.value.replace(/'/g, "\\'")}')">
                        <i class="bi bi-clipboard"></i> Copy Value
                    </button>
                    <button class="btn btn-sm btn-secondary mb-2 ms-2" onclick="formatJSON(${index})">
                        <i class="bi bi-code"></i> Format JSON
                    </button>
                    <pre id="storageValue${index}">${escapeHtml(item.value)}</pre>
                </div>
            </div>
        </div>`;
    });
    
    html += '</div>';
    content.innerHTML = html;
}

// Filter storage items
function filterStorage() {
    const filter = document.getElementById('storageFilter').value.toLowerCase();
    const filtered = storageData.filter(item => item.key.toLowerCase().includes(filter));
    renderStorage(filtered);
}

// Format JSON in storage value
function formatJSON(index) {
    const element = document.getElementById(`storageValue${index}`);
    const text = element.textContent;
    try {
        const parsed = JSON.parse(text);
        element.textContent = JSON.stringify(parsed, null, 2);
    } catch (e) {
        alert('Invalid JSON');
    }
}

// Copy to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        alert('Copied to clipboard!');
    }).catch(err => {
        console.error('Failed to copy:', err);
    });
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
