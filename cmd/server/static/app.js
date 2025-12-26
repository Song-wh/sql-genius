// SQL Genius - Frontend Application

const API_BASE = '';

// State
let currentQueryType = 'SELECT';
let currentSchema = null;
let isConnected = false;

// DOM Elements
const elements = {
    navItems: document.querySelectorAll('.nav-item'),
    tabContents: document.querySelectorAll('.tab-content'),
    typeButtons: document.querySelectorAll('.type-btn'),
    prompt: document.getElementById('prompt'),
    generateBtn: document.getElementById('generateBtn'),
    resultSection: document.getElementById('resultSection'),
    validateQuery: document.getElementById('validateQuery'),
    validateBtn: document.getElementById('validateBtn'),
    validateResultSection: document.getElementById('validateResultSection'),
    optimizeQuery: document.getElementById('optimizeQuery'),
    optimizeBtn: document.getElementById('optimizeBtn'),
    optimizeResultSection: document.getElementById('optimizeResultSection'),
    schemaInput: document.getElementById('schemaInput'),
    schemaDbType: document.getElementById('schemaDbType'),
    parseSchemaBtn: document.getElementById('parseSchemaBtn'),
    saveSchemaBtn: document.getElementById('saveSchemaBtn'),
    loadSchemaBtn: document.getElementById('loadSchemaBtn'),
    schemaFileInput: document.getElementById('schemaFileInput'),
    schemaView: document.getElementById('schemaView'),
    schemaTabs: document.querySelectorAll('.schema-tab'),
    connectionForm: document.getElementById('connectionForm'),
    disconnectBtn: document.getElementById('disconnectBtn'),
    connectionStatus: document.getElementById('connectionStatus'),
    status: document.getElementById('status'),
    dbType: document.getElementById('dbType'),
    dbPort: document.getElementById('dbPort'),
};

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    initQueryTypeSelector();
    initGenerateButton();
    initValidateButton();
    initOptimizeButton();
    initSchemaSection();
    initSchemaStorage();
    initConnectionForm();
    checkStatus();
});

// Navigation
function initNavigation() {
    elements.navItems.forEach(item => {
        item.addEventListener('click', () => {
            const tab = item.dataset.tab;
            
            elements.navItems.forEach(i => i.classList.remove('active'));
            item.classList.add('active');
            
            elements.tabContents.forEach(content => {
                content.classList.remove('active');
                if (content.id === `tab-${tab}`) {
                    content.classList.add('active');
                }
            });
        });
    });
}

// Query Type Selector
function initQueryTypeSelector() {
    elements.typeButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            elements.typeButtons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentQueryType = btn.dataset.type;
        });
    });
}

// Generate Button
function initGenerateButton() {
    elements.generateBtn.addEventListener('click', generateQuery);
    elements.prompt.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && e.ctrlKey) {
            generateQuery();
        }
    });
}

async function generateQuery() {
    const prompt = elements.prompt.value.trim();
    if (!prompt) {
        showError(elements.resultSection, '쿼리 요청을 입력해주세요');
        return;
    }
    
    if (!currentSchema) {
        showError(elements.resultSection, '먼저 스키마를 로드하거나 DB에 연결해주세요');
        return;
    }
    
    showLoading(elements.resultSection);
    elements.generateBtn.disabled = true;
    
    try {
        const response = await fetch(`${API_BASE}/api/generate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                prompt: prompt,
                query_type: currentQueryType,
                schema: currentSchema
            })
        });
        
        const result = await response.json();
        
        if (result.success) {
            showQueryResult(elements.resultSection, result.data);
        } else {
            showError(elements.resultSection, result.error);
        }
    } catch (error) {
        showError(elements.resultSection, '서버 연결 실패: ' + error.message);
    } finally {
        elements.generateBtn.disabled = false;
    }
}

// Validate Button
function initValidateButton() {
    elements.validateBtn.addEventListener('click', validateQuery);
}

async function validateQuery() {
    const query = elements.validateQuery.value.trim();
    if (!query) {
        showError(elements.validateResultSection, 'SQL 쿼리를 입력해주세요');
        return;
    }
    
    if (!currentSchema) {
        showError(elements.validateResultSection, '먼저 스키마를 로드하거나 DB에 연결해주세요');
        return;
    }
    
    showLoading(elements.validateResultSection);
    elements.validateBtn.disabled = true;
    
    try {
        const response = await fetch(`${API_BASE}/api/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query })
        });
        
        const result = await response.json();
        
        if (result.success) {
            showValidationResult(elements.validateResultSection, result.data);
        } else {
            showError(elements.validateResultSection, result.error);
        }
    } catch (error) {
        showError(elements.validateResultSection, '서버 연결 실패: ' + error.message);
    } finally {
        elements.validateBtn.disabled = false;
    }
}

function showValidationResult(container, data) {
    const scoreClass = data.score >= 70 ? 'score-high' : (data.score >= 40 ? 'score-medium' : 'score-low');
    const scoreLabel = data.score >= 70 ? '우수' : (data.score >= 40 ? '보통' : '개선 필요');
    
    let issuesHTML = '';
    if (data.issues && data.issues.length > 0) {
        issuesHTML = `
            <div class="issues-section">
                <h4>🔍 발견된 문제점</h4>
                ${data.issues.map(issue => `
                    <div class="issue-item ${issue.type}">
                        <span class="issue-icon">${issue.type === 'error' ? '❌' : (issue.type === 'warning' ? '⚠️' : 'ℹ️')}</span>
                        <div class="issue-content">
                            <div class="issue-message">${escapeHtml(issue.message)}</div>
                            ${issue.location ? `<div class="issue-location">위치: ${escapeHtml(issue.location)}</div>` : ''}
                            ${issue.suggestion ? `<div class="issue-suggestion">💡 ${escapeHtml(issue.suggestion)}</div>` : ''}
                        </div>
                    </div>
                `).join('')}
            </div>
        `;
    }
    
    let indexHTML = '';
    if (data.index_usage && data.index_usage.length > 0) {
        indexHTML = `
            <div class="index-usage">
                <h4>📊 사용 가능한 인덱스</h4>
                <div class="index-list">
                    ${data.index_usage.map(idx => `<span class="index-tag">${escapeHtml(idx)}</span>`).join('')}
                </div>
            </div>
        `;
    }
    
    let optimizedHTML = '';
    if (data.optimized_query && data.optimized_query !== data.original_query) {
        optimizedHTML = `
            <div class="optimized-section">
                <h4>✨ 최적화된 쿼리 제안</h4>
                <div class="sql-code" style="position: relative;">
                    <button class="copy-btn" onclick="copyOptimizedSQL()">복사</button>
                    <pre id="optimizedSqlContent">${highlightSQL(data.optimized_query)}</pre>
                </div>
            </div>
        `;
    }
    
    let suggestionsHTML = '';
    if (data.suggestions && data.suggestions.length > 0) {
        suggestionsHTML = `
            <div class="result-tips">
                <h4>🚀 개선 제안</h4>
                <ul>
                    ${data.suggestions.map(s => `<li>${escapeHtml(s)}</li>`).join('')}
                </ul>
            </div>
        `;
    }
    
    container.innerHTML = `
        <div class="result-content">
            <div class="score-card">
                <div class="score-circle ${scoreClass}">${data.score}</div>
                <div class="score-info">
                    <h3>성능 점수: ${scoreLabel}</h3>
                    <p>예상 실행 시간: ${data.estimated_time || '분석 중'}</p>
                </div>
                <span class="validity-badge ${data.is_valid ? 'valid' : 'invalid'}">
                    ${data.is_valid ? '✓ 유효한 쿼리' : '✗ 문법 오류'}
                </span>
            </div>
            
            <div class="result-header">
                <h3>📝 원본 쿼리</h3>
                <span class="result-time">⏱️ AI 분석: ${data.ai_response_time}ms</span>
            </div>
            <div class="sql-code">
                <pre>${highlightSQL(data.original_query)}</pre>
            </div>
            
            ${issuesHTML}
            ${indexHTML}
            ${optimizedHTML}
            
            ${data.execution_plan ? `
                <div class="execution-plan">
                    <strong>📋 예상 실행 계획:</strong><br>
                    ${escapeHtml(data.execution_plan)}
                </div>
            ` : ''}
            
            ${suggestionsHTML}
        </div>
    `;
}

function copyOptimizedSQL() {
    const content = document.getElementById('optimizedSqlContent');
    if (content) {
        navigator.clipboard.writeText(content.textContent).then(() => {
            const btns = document.querySelectorAll('.optimized-section .copy-btn');
            btns.forEach(btn => {
                btn.textContent = '복사됨!';
                setTimeout(() => btn.textContent = '복사', 2000);
            });
        });
    }
}

// Optimize Button
function initOptimizeButton() {
    elements.optimizeBtn.addEventListener('click', optimizeQuery);
}

async function optimizeQuery() {
    const query = elements.optimizeQuery.value.trim();
    if (!query) {
        showError(elements.optimizeResultSection, 'SQL 쿼리를 입력해주세요');
        return;
    }
    
    showLoading(elements.optimizeResultSection);
    elements.optimizeBtn.disabled = true;
    
    try {
        const response = await fetch(`${API_BASE}/api/optimize`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query })
        });
        
        const result = await response.json();
        
        if (result.success) {
            showQueryResult(elements.optimizeResultSection, result.data, '최적화된 쿼리');
        } else {
            showError(elements.optimizeResultSection, result.error);
        }
    } catch (error) {
        showError(elements.optimizeResultSection, '서버 연결 실패: ' + error.message);
    } finally {
        elements.optimizeBtn.disabled = false;
    }
}

// Schema Section
function initSchemaSection() {
    elements.schemaTabs.forEach(tab => {
        tab.addEventListener('click', () => {
            elements.schemaTabs.forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            
            const type = tab.dataset.schemaType;
            if (type === 'ddl') {
                elements.schemaInput.placeholder = `CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`;
            } else {
                elements.schemaInput.placeholder = `{
    "database": "mydb",
    "db_type": "mysql",
    "tables": [
        {
            "name": "users",
            "columns": [
                {"name": "id", "type": "INT", "is_pk": true},
                {"name": "name", "type": "VARCHAR(100)"}
            ]
        }
    ]
}`;
            }
        });
    });
    
    elements.parseSchemaBtn.addEventListener('click', parseSchema);
}

async function parseSchema() {
    const input = elements.schemaInput.value.trim();
    if (!input) {
        showError(elements.schemaView, 'DDL 또는 JSON을 입력해주세요');
        return;
    }
    
    const activeTab = document.querySelector('.schema-tab.active');
    const schemaType = activeTab.dataset.schemaType;
    
    showLoading(elements.schemaView);
    elements.parseSchemaBtn.disabled = true;
    
    try {
        const body = schemaType === 'ddl' 
            ? { ddl: input, db_type: elements.schemaDbType.value }
            : { json: input };
        
        const response = await fetch(`${API_BASE}/api/schema/parse`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        
        const result = await response.json();
        
        if (result.success) {
            currentSchema = result.data;
            renderSchema(result.data);
            updateStatus(true, `${result.data.tables.length}개 테이블 로드됨`);
        } else {
            showError(elements.schemaView, result.error);
        }
    } catch (error) {
        showError(elements.schemaView, '서버 연결 실패: ' + error.message);
    } finally {
        elements.parseSchemaBtn.disabled = false;
    }
}

// Schema Storage (Save/Load)
function initSchemaStorage() {
    elements.saveSchemaBtn.addEventListener('click', saveSchema);
    elements.loadSchemaBtn.addEventListener('click', () => elements.schemaFileInput.click());
    elements.schemaFileInput.addEventListener('change', loadSchemaFromFile);
}

function saveSchema() {
    if (!currentSchema) {
        alert('저장할 스키마가 없습니다. 먼저 스키마를 로드해주세요.');
        return;
    }
    
    const schemaJSON = JSON.stringify(currentSchema, null, 2);
    const blob = new Blob([schemaJSON], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    
    const a = document.createElement('a');
    a.href = url;
    a.download = `schema_${currentSchema.database || 'export'}_${new Date().toISOString().slice(0,10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    // 버튼 피드백
    const originalText = elements.saveSchemaBtn.innerHTML;
    elements.saveSchemaBtn.innerHTML = '<span class="btn-icon">✅</span><span>저장됨!</span>';
    setTimeout(() => {
        elements.saveSchemaBtn.innerHTML = originalText;
    }, 2000);
}

async function loadSchemaFromFile(event) {
    const file = event.target.files[0];
    if (!file) return;
    
    try {
        const text = await file.text();
        const schema = JSON.parse(text);
        
        // 서버에 스키마 전송
        const response = await fetch(`${API_BASE}/api/schema/parse`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ json: text })
        });
        
        const result = await response.json();
        
        if (result.success) {
            currentSchema = result.data;
            renderSchema(result.data);
            updateStatus(true, `${result.data.tables.length}개 테이블 로드됨`);
            
            // 입력창에도 JSON 표시
            elements.schemaInput.value = text;
            
            // JSON 탭 활성화
            elements.schemaTabs.forEach(t => t.classList.remove('active'));
            document.querySelector('.schema-tab[data-schema-type="json"]').classList.add('active');
            
            alert(`스키마 파일을 성공적으로 불러왔습니다!\n테이블 ${result.data.tables.length}개 로드됨`);
        } else {
            alert('스키마 파일 파싱 실패: ' + result.error);
        }
    } catch (error) {
        alert('파일 읽기 실패: ' + error.message);
    }
    
    // 파일 입력 초기화 (같은 파일 다시 선택 가능하도록)
    event.target.value = '';
}

function renderSchema(schema) {
    let html = '';
    
    for (const table of schema.tables) {
        const columnCount = table.columns ? table.columns.length : 0;
        const indexCount = table.indexes ? table.indexes.length : 0;
        const fkCount = table.foreign_keys ? table.foreign_keys.length : 0;
        
        html += `
            <div class="table-card" data-table="${escapeHtml(table.name)}">
                <div class="table-header" onclick="toggleTableDetail('${escapeHtml(table.name)}')">
                    <span class="table-icon">📋</span>
                    <span class="table-name">${escapeHtml(table.name)}</span>
                    <span class="table-meta">${columnCount}개 컬럼</span>
                    <button class="sample-btn" onclick="event.stopPropagation(); loadSampleData('${escapeHtml(table.name)}')">
                        👁️ 데이터 보기
                    </button>
                </div>
                <div class="table-columns" id="cols-${escapeHtml(table.name)}">
        `;
        
        if (table.columns) {
            for (const col of table.columns) {
                const badges = [];
                if (col.is_pk) badges.push('<span class="badge badge-pk">PK</span>');
                if (col.is_fk) badges.push('<span class="badge badge-fk">FK</span>');
                if (col.is_unique) badges.push('<span class="badge badge-unique">UQ</span>');
                if (col.is_auto_incr) badges.push('<span class="badge badge-auto">AUTO</span>');
                
                const nullable = col.nullable ? 'NULL' : 'NOT NULL';
                const defaultVal = col.default ? `= ${col.default}` : '';
                
                html += `
                    <div class="column-item">
                        <span class="column-name">${escapeHtml(col.name)}</span>
                        <span class="column-type">${escapeHtml(col.type)}</span>
                        <span class="column-nullable">${nullable}</span>
                        <span class="column-default">${escapeHtml(defaultVal)}</span>
                        <div class="column-badges">${badges.join('')}</div>
                    </div>
                `;
            }
        }
        
        // 인덱스 정보
        if (table.indexes && table.indexes.length > 0) {
            html += `<div class="table-section"><h5>📊 인덱스 (${indexCount})</h5>`;
            for (const idx of table.indexes) {
                const unique = idx.is_unique ? '🔒 UNIQUE' : '';
                html += `<div class="index-item">${escapeHtml(idx.name)} (${idx.columns ? idx.columns.join(', ') : ''}) ${unique}</div>`;
            }
            html += '</div>';
        }
        
        // 외래키 정보
        if (table.foreign_keys && table.foreign_keys.length > 0) {
            html += `<div class="table-section"><h5>🔗 외래키 (${fkCount})</h5>`;
            for (const fk of table.foreign_keys) {
                html += `<div class="fk-item">${escapeHtml(fk.column)} → ${escapeHtml(fk.ref_table)}.${escapeHtml(fk.ref_column)}</div>`;
            }
            html += '</div>';
        }
        
        html += '</div></div>';
    }
    
    // 샘플 데이터 모달
    html += `
        <div id="sampleModal" class="modal" style="display:none;">
            <div class="modal-content">
                <div class="modal-header">
                    <h3 id="sampleModalTitle">테이블 데이터</h3>
                    <button class="modal-close" onclick="closeSampleModal()">✕</button>
                </div>
                <div class="modal-body" id="sampleModalBody">
                    로딩 중...
                </div>
            </div>
        </div>
    `;
    
    elements.schemaView.innerHTML = html;
}

function toggleTableDetail(tableName) {
    const cols = document.getElementById(`cols-${tableName}`);
    if (cols) {
        cols.classList.toggle('expanded');
    }
}

async function loadSampleData(tableName) {
    const modal = document.getElementById('sampleModal');
    const title = document.getElementById('sampleModalTitle');
    const body = document.getElementById('sampleModalBody');
    
    modal.style.display = 'flex';
    title.textContent = `📋 ${tableName} 샘플 데이터`;
    body.innerHTML = '<div class="loading"><div class="spinner"></div><span>데이터 로딩 중...</span></div>';
    
    try {
        const response = await fetch(`${API_BASE}/api/schema/sample?table=${encodeURIComponent(tableName)}&limit=20`);
        const result = await response.json();
        
        if (result.success) {
            renderSampleData(result.data, body);
        } else {
            body.innerHTML = `<div class="error-message">❌ ${escapeHtml(result.error)}</div>`;
        }
    } catch (error) {
        body.innerHTML = `<div class="error-message">❌ 데이터 로딩 실패: ${escapeHtml(error.message)}</div>`;
    }
}

function renderSampleData(data, container) {
    if (!data.rows || data.rows.length === 0) {
        container.innerHTML = '<div class="empty-message">📭 테이블에 데이터가 없습니다</div>';
        return;
    }
    
    let html = `<div class="sample-info">총 ${data.count}개 행 (최대 20개 표시)</div>`;
    html += '<div class="sample-table-wrapper"><table class="sample-table"><thead><tr>';
    
    for (const col of data.columns) {
        html += `<th>${escapeHtml(col)}</th>`;
    }
    html += '</tr></thead><tbody>';
    
    for (const row of data.rows) {
        html += '<tr>';
        for (const cell of row) {
            let cellValue = cell;
            if (cell === null) {
                cellValue = '<span class="null-value">NULL</span>';
            } else if (typeof cell === 'object') {
                cellValue = JSON.stringify(cell);
            } else {
                cellValue = escapeHtml(String(cell));
                // 긴 텍스트 자르기
                if (cellValue.length > 50) {
                    cellValue = cellValue.substring(0, 50) + '...';
                }
            }
            html += `<td>${cellValue}</td>`;
        }
        html += '</tr>';
    }
    
    html += '</tbody></table></div>';
    container.innerHTML = html;
}

function closeSampleModal() {
    document.getElementById('sampleModal').style.display = 'none';
}

// Connection Form
function initConnectionForm() {
    elements.connectionForm.addEventListener('submit', handleConnect);
    elements.disconnectBtn.addEventListener('click', handleDisconnect);
    
    // Auto-update port based on DB type
    elements.dbType.addEventListener('change', () => {
        const ports = {
            mysql: 3306,
            postgresql: 5432,
            oracle: 1521,
            sqlserver: 1433
        };
        elements.dbPort.value = ports[elements.dbType.value] || 3306;
    });
}

async function handleConnect(e) {
    e.preventDefault();
    
    const formData = {
        type: elements.dbType.value,
        host: document.getElementById('dbHost').value,
        port: parseInt(document.getElementById('dbPort').value),
        user: document.getElementById('dbUser').value,
        password: document.getElementById('dbPassword').value,
        database: document.getElementById('dbName').value
    };
    
    showLoading(elements.connectionStatus);
    
    try {
        const response = await fetch(`${API_BASE}/api/connect`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(formData)
        });
        
        const result = await response.json();
        
        if (result.success) {
            isConnected = true;
            currentSchema = result.data.schema;
            elements.disconnectBtn.disabled = false;
            
            showConnectionSuccess(result.data.schema);
            renderSchema(result.data.schema);
            updateStatus(true, `${result.data.schema.database} 연결됨`);
        } else {
            showError(elements.connectionStatus, result.error);
        }
    } catch (error) {
        showError(elements.connectionStatus, '서버 연결 실패: ' + error.message);
    }
}

async function handleDisconnect() {
    try {
        await fetch(`${API_BASE}/api/disconnect`, { method: 'POST' });
        
        isConnected = false;
        currentSchema = null;
        elements.disconnectBtn.disabled = true;
        
        elements.connectionStatus.innerHTML = `
            <div class="result-placeholder">
                <div class="placeholder-icon">🔌</div>
                <p>연결이 해제되었습니다</p>
            </div>
        `;
        
        elements.schemaView.innerHTML = `
            <div class="result-placeholder">
                <div class="placeholder-icon">📋</div>
                <p>스키마를 로드하면 여기에 표시됩니다</p>
            </div>
        `;
        
        updateStatus(false);
    } catch (error) {
        console.error('Disconnect error:', error);
    }
}

function showConnectionSuccess(schema) {
    elements.connectionStatus.innerHTML = `
        <div class="result-content">
            <div class="result-header">
                <h3>✅ 연결 성공</h3>
            </div>
            <div class="result-explanation">
                <h4>연결 정보</h4>
                <p>
                    데이터베이스: <strong>${escapeHtml(schema.database)}</strong><br>
                    타입: <strong>${escapeHtml(schema.db_type)}</strong><br>
                    테이블 수: <strong>${schema.tables.length}</strong>개
                </p>
            </div>
        </div>
    `;
}

// Status Check
async function checkStatus() {
    try {
        const response = await fetch(`${API_BASE}/api/status`);
        const result = await response.json();
        
        if (result.success) {
            const data = result.data;
            
            if (data.schema_loaded) {
                updateStatus(true, `${data.tables_count}개 테이블`);
            } else if (data.ai_available) {
                updateStatus(false, `${data.ai_provider} 준비됨`);
            }
        }
    } catch (error) {
        updateStatus(false, '서버 연결 실패');
    }
}

function updateStatus(connected, text = '') {
    const dot = elements.status.querySelector('.status-dot');
    const span = elements.status.querySelector('span');
    
    if (connected) {
        dot.classList.add('connected');
        span.textContent = text || '연결됨';
    } else {
        dot.classList.remove('connected');
        span.textContent = text || '연결 대기 중';
    }
}

// UI Helpers
function showLoading(container) {
    container.innerHTML = `
        <div class="loading">
            <div class="spinner"></div>
            <span>처리 중...</span>
        </div>
    `;
}

function showError(container, message) {
    container.innerHTML = `
        <div class="result-placeholder" style="color: var(--error);">
            <div class="placeholder-icon">❌</div>
            <p>${escapeHtml(message)}</p>
        </div>
    `;
}

function showQueryResult(container, data, title = '생성된 쿼리') {
    const highlightedSQL = highlightSQL(data.query);
    
    let tipsHTML = '';
    if (data.tips && data.tips.length > 0) {
        tipsHTML = `
            <div class="result-tips">
                <h4>🚀 최적화 팁</h4>
                <ul>
                    ${data.tips.map(tip => `<li>${escapeHtml(tip)}</li>`).join('')}
                </ul>
            </div>
        `;
    }
    
    container.innerHTML = `
        <div class="result-content">
            <div class="result-header">
                <h3>📝 ${escapeHtml(title)}</h3>
                <span class="result-time">⏱️ ${data.execute_time}ms</span>
            </div>
            <div class="sql-code" style="position: relative;">
                <button class="copy-btn" onclick="copySQL()">복사</button>
                <pre id="sqlContent">${highlightedSQL}</pre>
            </div>
            ${data.explanation ? `
                <div class="result-explanation">
                    <h4>💡 설명</h4>
                    <p>${escapeHtml(data.explanation)}</p>
                </div>
            ` : ''}
            ${tipsHTML}
        </div>
    `;
}

function highlightSQL(sql) {
    const keywords = [
        'SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'NOT', 'IN', 'LIKE', 'BETWEEN',
        'JOIN', 'LEFT', 'RIGHT', 'INNER', 'OUTER', 'FULL', 'CROSS', 'ON',
        'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'OFFSET', 'AS', 'DISTINCT',
        'INSERT', 'INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE', 'TRUNCATE',
        'CREATE', 'ALTER', 'DROP', 'TABLE', 'INDEX', 'PRIMARY KEY', 'FOREIGN KEY',
        'REFERENCES', 'CONSTRAINT', 'DEFAULT', 'NULL', 'NOT NULL', 'AUTO_INCREMENT',
        'UNIQUE', 'CHECK', 'CASCADE', 'RESTRICT', 'IF', 'EXISTS', 'UNION', 'ALL',
        'ASC', 'DESC', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END', 'IS', 'TRUE', 'FALSE'
    ];
    
    let highlighted = escapeHtml(sql);
    
    // Highlight keywords
    keywords.forEach(kw => {
        const regex = new RegExp(`\\b(${kw})\\b`, 'gi');
        highlighted = highlighted.replace(regex, '<span class="sql-keyword">$1</span>');
    });
    
    // Highlight strings
    highlighted = highlighted.replace(/'([^']*)'/g, '<span class="sql-string">\'$1\'</span>');
    
    // Highlight numbers
    highlighted = highlighted.replace(/\b(\d+)\b/g, '<span class="sql-number">$1</span>');
    
    // Highlight functions
    const functions = ['COUNT', 'SUM', 'AVG', 'MIN', 'MAX', 'CONCAT', 'SUBSTRING', 'UPPER', 'LOWER', 'NOW', 'DATE', 'YEAR', 'MONTH', 'DAY'];
    functions.forEach(fn => {
        const regex = new RegExp(`\\b(${fn})\\s*\\(`, 'gi');
        highlighted = highlighted.replace(regex, '<span class="sql-function">$1</span>(');
    });
    
    return highlighted;
}

function copySQL() {
    const sqlContent = document.getElementById('sqlContent');
    const text = sqlContent.textContent || sqlContent.innerText;
    
    navigator.clipboard.writeText(text).then(() => {
        const btn = document.querySelector('.copy-btn');
        btn.textContent = '복사됨!';
        setTimeout(() => btn.textContent = '복사', 2000);
    });
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Make copySQL global
window.copySQL = copySQL;

