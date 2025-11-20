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
            initSwaggerUI();
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

// Initialize Swagger UI
function initSwaggerUI() {
    if (openAPIData) return; // Already initialized
    
    openAPIData = true; // Mark as initializing
    
    SwaggerUIBundle({
        url: `${API_BASE}/sys/plugins/catalog/openapi`,
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIStandalonePreset
        ],
        plugins: [
            SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout",
        requestInterceptor: (request) => {
            // Ensure requests go through our API base
            if (!request.url.startsWith('http')) {
                request.url = `${window.location.origin}${request.url}`;
            }
            return request;
        }
    });
}

// Load OpenAPI specification (legacy function for dashboard)
async function loadOpenAPI() {
    const content = document.getElementById('openapiContent');
    content.innerHTML = '<div class="spinner-border spinner-border-sm" role="status"></div> Loading...';
    
    try {
        const data = await fetch(`${API_BASE}/sys/plugins/catalog/openapi`).then(r => r.json());
        openAPIData = data;
        
        let html = '';
        
        if (data.info) {
            html += '<div class="mb-4 pb-3 border-bottom border-secondary">';
            html += `<h4><i class="bi bi-book"></i> ${data.info.title || 'API Documentation'}</h4>`;
            if (data.info.version) html += `<p class="mb-1"><span class="badge bg-info">v${data.info.version}</span></p>`;
            if (data.info.description) html += `<p class="text-muted">${data.info.description}</p>`;
            html += '</div>';
        }
        
        if (data.paths) {
            // Group paths by tag or show all
            const paths = Object.entries(data.paths);
            
            paths.forEach(([path, pathData], index) => {
                for (const [method, operation] of Object.entries(pathData)) {
                    html += renderEndpoint(path, method, operation, index);
                }
            });
        }
        
        // Add raw JSON at the bottom
        html += '<div class="mt-5 pt-4 border-top border-secondary">';
        html += '<h5>Raw OpenAPI Specification</h5>';
        html += '<button class="btn btn-sm btn-secondary mb-2" onclick="copyToClipboard(JSON.stringify(openAPIData, null, 2))">';
        html += '<i class="bi bi-clipboard"></i> Copy to Clipboard</button>';
        html += `<pre>${JSON.stringify(data, null, 2)}</pre>`;
        html += '</div>';
        
        content.innerHTML = html;
    } catch (error) {
        content.innerHTML = `<div class="alert alert-danger">Error loading OpenAPI: ${error.message}</div>`;
    }
}

// Render individual endpoint
function renderEndpoint(path, method, operation, index) {
    const methodLower = method.toLowerCase();
    const endpointId = `endpoint-${index}-${methodLower}`;
    
    let html = `<div class="openapi-endpoint" id="${endpointId}">`;
    
    // Header
    html += `<div class="endpoint-header" onclick="toggleEndpoint('${endpointId}')">` ;
    html += `<span class="badge method-${methodLower} method-badge">${method.toUpperCase()}</span>`;
    html += `<code class="endpoint-path flex-grow-1">${path}</code>`;
    if (operation.summary) {
        html += `<span class="text-muted small">${operation.summary}</span>`;
    }
    html += '<i class="bi bi-chevron-down ms-2" id="' + endpointId + '-icon"></i>';
    html += '</div>';
    
    // Body (hidden by default)
    html += `<div class="endpoint-body d-none" id="${endpointId}-body">`;
    
    if (operation.description) {
        html += `<p class="text-muted">${operation.description}</p>`;
    }
    
    // Parameters section
    const pathParams = [];
    const queryParams = [];
    const headerParams = [];
    
    if (operation.parameters) {
        operation.parameters.forEach(param => {
            if (param.in === 'path') pathParams.push(param);
            else if (param.in === 'query') queryParams.push(param);
            else if (param.in === 'header') headerParams.push(param);
        });
    }
    
    // Path Parameters
    if (pathParams.length > 0) {
        html += '<h6 class="mt-3">Path Parameters</h6>';
        html += '<table class="table table-sm param-table">';
        html += '<thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Value</th></tr></thead><tbody>';
        pathParams.forEach(param => {
            html += '<tr>';
            html += `<td><code>${param.name}</code></td>`;
            html += `<td>${param.schema?.type || 'string'}</td>`;
            html += `<td>${param.required ? '<span class="required-marker">*</span> Yes' : 'No'}</td>`;
            html += `<td><input type="text" class="form-control form-control-sm param-input" ` +
                   `id="${endpointId}-path-${param.name}" placeholder="${param.description || param.name}" /></td>`;
            html += '</tr>';
        });
        html += '</tbody></table>';
    }
    
    // Query Parameters
    if (queryParams.length > 0) {
        html += '<h6 class="mt-3">Query Parameters</h6>';
        html += '<table class="table table-sm param-table">';
        html += '<thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Value</th></tr></thead><tbody>';
        queryParams.forEach(param => {
            html += '<tr>';
            html += `<td><code>${param.name}</code></td>`;
            html += `<td>${param.schema?.type || 'string'}</td>`;
            html += `<td>${param.required ? '<span class="required-marker">*</span> Yes' : 'No'}</td>`;
            html += `<td><input type="text" class="form-control form-control-sm param-input" ` +
                   `id="${endpointId}-query-${param.name}" placeholder="${param.description || param.name}" /></td>`;
            html += '</tr>';
        });
        html += '</tbody></table>';
    }
    
    // Request Body
    if (operation.requestBody) {
        html += '<h6 class="mt-3">Request Body</h6>';
        
        const content = operation.requestBody.content;
        if (content && content['application/json']) {
            const schema = content['application/json'].schema;
            
            // Show example or schema
            let exampleBody = '{}';
            if (content['application/json'].example) {
                exampleBody = JSON.stringify(content['application/json'].example, null, 2);
            } else if (schema) {
                exampleBody = generateExample(schema);
            }
            
            html += `<textarea class="form-control" id="${endpointId}-body" rows="8" ` +
                   `style="font-family: monospace; font-size: 0.875rem;">${exampleBody}</textarea>`;
            
            if (schema) {
                html += '<details class="mt-2"><summary class="text-muted small" style="cursor: pointer;">View Schema</summary>';
                html += renderSchema(schema);
                html += '</details>';
            }
        }
    }
    
    // Execute button
    html += '<div class="mt-3 d-flex gap-2">';
    html += `<button class="btn btn-execute" onclick="executeRequest('${endpointId}', '${path}', '${method}')">`;
    html += '<i class="bi bi-play-fill"></i> Execute</button>';
    html += `<button class="btn btn-clear" onclick="clearResponse('${endpointId}')">`;
    html += '<i class="bi bi-x-circle"></i> Clear</button>';
    html += '</div>';
    
    // Response section
    html += `<div id="${endpointId}-response" class="d-none"></div>`;
    
    html += '</div>'; // end endpoint-body
    html += '</div>'; // end openapi-endpoint
    
    return html;
}

// Generate example from schema
function generateExample(schema) {
    if (!schema) return '{}';
    
    if (schema.example) return JSON.stringify(schema.example, null, 2);
    if (schema.type === 'string') return '"string"';
    if (schema.type === 'number' || schema.type === 'integer') return '0';
    if (schema.type === 'boolean') return 'false';
    if (schema.type === 'array') {
        const items = schema.items ? generateExample(schema.items) : '{}';
        return JSON.stringify([JSON.parse(items)], null, 2);
    }
    if (schema.type === 'object' && schema.properties) {
        const obj = {};
        for (const [key, prop] of Object.entries(schema.properties)) {
            if (prop.example !== undefined) {
                obj[key] = prop.example;
            } else if (prop.type === 'string') {
                obj[key] = prop.enum ? prop.enum[0] : 'string';
            } else if (prop.type === 'number' || prop.type === 'integer') {
                obj[key] = 0;
            } else if (prop.type === 'boolean') {
                obj[key] = false;
            } else if (prop.type === 'array') {
                obj[key] = [];
            } else if (prop.type === 'object') {
                obj[key] = {};
            } else {
                obj[key] = null;
            }
        }
        return JSON.stringify(obj, null, 2);
    }
    
    return '{}';
}

// Render schema
function renderSchema(schema, depth = 0) {
    if (!schema) return '';
    
    let html = '<div class="schema-object">';
    
    if (schema.type === 'object' && schema.properties) {
        html += '<div class="schema-property">{</div>';
        for (const [key, prop] of Object.entries(schema.properties)) {
            const required = schema.required && schema.required.includes(key);
            html += `<div class="schema-property ms-3">`;
            html += `<span class="text-info">${key}</span>${required ? '<span class="required-marker">*</span>' : ''}: `;
            html += `<span class="text-muted">${prop.type || 'any'}</span>`;
            if (prop.description) html += ` <span class="text-muted">// ${prop.description}</span>`;
            html += '</div>';
            
            if (prop.type === 'object' && prop.properties) {
                html += renderSchema(prop, depth + 1);
            }
        }
        html += '<div class="schema-property">}</div>';
    } else if (schema.type === 'array') {
        html += `<div class="schema-property">Array of ${schema.items?.type || 'any'}</div>`;
    } else {
        html += `<div class="schema-property">${schema.type || 'any'}</div>`;
    }
    
    html += '</div>';
    return html;
}

// Toggle endpoint visibility
function toggleEndpoint(endpointId) {
    const body = document.getElementById(`${endpointId}-body`);
    const icon = document.getElementById(`${endpointId}-icon`);
    const header = document.querySelector(`#${endpointId} .endpoint-header`);
    
    if (body.classList.contains('d-none')) {
        body.classList.remove('d-none');
        icon.classList.remove('bi-chevron-down');
        icon.classList.add('bi-chevron-up');
        header.classList.add('expanded');
    } else {
        body.classList.add('d-none');
        icon.classList.remove('bi-chevron-up');
        icon.classList.add('bi-chevron-down');
        header.classList.remove('expanded');
    }
}

// Execute API request
async function executeRequest(endpointId, path, method) {
    const responseDiv = document.getElementById(`${endpointId}-response`);
    responseDiv.classList.remove('d-none');
    responseDiv.innerHTML = '<div class="response-section"><div class="spinner-border spinner-border-sm" role="status"></div> Executing...</div>';
    
    try {
        // Build the URL with path parameters
        let url = path;
        const pathParams = document.querySelectorAll(`[id^="${endpointId}-path-"]`);
        pathParams.forEach(input => {
            const paramName = input.id.replace(`${endpointId}-path-`, '');
            const value = input.value || `{${paramName}}`;
            url = url.replace(`{${paramName}}`, encodeURIComponent(value));
        });
        
        // Add query parameters
        const queryParams = document.querySelectorAll(`[id^="${endpointId}-query-"]`);
        const queryString = [];
        queryParams.forEach(input => {
            if (input.value) {
                const paramName = input.id.replace(`${endpointId}-query-`, '');
                queryString.push(`${paramName}=${encodeURIComponent(input.value)}`);
            }
        });
        if (queryString.length > 0) {
            url += '?' + queryString.join('&');
        }
        
        // Build request options
        const options = {
            method: method.toUpperCase(),
            headers: {
                'Content-Type': 'application/json'
            }
        };
        
        // Add request body if present
        const bodyTextarea = document.getElementById(`${endpointId}-body`);
        if (bodyTextarea && bodyTextarea.value && method.toUpperCase() !== 'GET') {
            try {
                JSON.parse(bodyTextarea.value); // Validate JSON
                options.body = bodyTextarea.value;
            } catch (e) {
                throw new Error('Invalid JSON in request body');
            }
        }
        
        // Generate curl command
        const curlCmd = generateCurl(url, options);
        
        // Execute request
        const startTime = Date.now();
        const response = await fetch(`${API_BASE}${url}`, options);
        const duration = Date.now() - startTime;
        
        // Get response body
        const contentType = response.headers.get('content-type');
        let responseBody;
        if (contentType && contentType.includes('application/json')) {
            responseBody = await response.json();
        } else {
            responseBody = await response.text();
        }
        
        // Render response
        renderResponse(responseDiv, response.status, responseBody, duration, curlCmd);
        
    } catch (error) {
        responseDiv.innerHTML = `<div class="response-section"><div class="alert alert-danger mb-0">` +
                              `<strong>Error:</strong> ${error.message}</div></div>`;
    }
}

// Generate curl command
function generateCurl(url, options) {
    let cmd = `curl -X ${options.method} '${window.location.origin}${API_BASE}${url}'`;
    
    if (options.headers) {
        for (const [key, value] of Object.entries(options.headers)) {
            cmd += ` \\
  -H '${key}: ${value}'`;
        }
    }
    
    if (options.body) {
        cmd += ` \\
  -d '${options.body.replace(/'/g, "'\\''")}' `;
    }
    
    return cmd;
}

// Render response
function renderResponse(container, status, body, duration, curlCmd) {
    let statusClass = 'response-2xx';
    if (status >= 400 && status < 500) statusClass = 'response-4xx';
    else if (status >= 500) statusClass = 'response-5xx';
    
    let html = '<div class="response-section">';
    html += '<h6><i class="bi bi-arrow-return-left"></i> Response</h6>';
    html += `<div class="mb-2">`;
    html += `<span class="response-code ${statusClass}">${status}</span>`;
    html += `<span class="text-muted ms-2">${duration}ms</span>`;
    html += '</div>';
    
    // Response body
    html += '<h6 class="mt-3">Response Body</h6>';
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body, null, 2);
    html += `<pre>${escapeHtml(bodyStr)}</pre>`;
    
    // Curl command
    html += '<details class="mt-3"><summary class="text-muted" style="cursor: pointer;"><i class="bi bi-terminal"></i> Curl Command</summary>';
    html += `<div class="curl-command mt-2">${escapeHtml(curlCmd)}</div>`;
    html += '<button class="btn btn-sm btn-secondary mt-2" onclick="copyToClipboard(\`' + 
           curlCmd.replace(/`/g, '\\`') + '\`)">';
    html += '<i class="bi bi-clipboard"></i> Copy Curl</button>';
    html += '</details>';
    
    html += '</div>';
    container.innerHTML = html;
}

// Clear response
function clearResponse(endpointId) {
    const responseDiv = document.getElementById(`${endpointId}-response`);
    responseDiv.innerHTML = '';
    responseDiv.classList.add('d-none');
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
