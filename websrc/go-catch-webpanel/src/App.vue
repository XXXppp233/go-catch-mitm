<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { fileTypeFromBuffer } from 'file-type';
import { hexy } from 'hexy';
import isUtf8 from 'is-utf8';

interface TrafficRecord {
  created_at: string;
  method: string;
  src_ip: string;
  dst_host: string;
  dst_ip: string;
  url: string;
  status: number;
  req_headers: string; // JSON string of Record<string, string[]>
  req_body: string | null;
  resp_headers: string; // JSON string of Record<string, string[]>
  resp_body_path: string;
  user_agent: string;
  // UI state
  isExpanded?: boolean;
  showReqBody?: boolean;
  parsedReqBody?: string;
  reqBodyLoading?: boolean;
}

const records = ref<TrafficRecord[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

// Sorting & Filtering State
const timeSortOrder = ref<'desc' | 'asc'>('desc');
const filterMethod = ref('');
const filterStatus = ref('');
const filterHost = ref('');
const filterUrl = ref('');

const fetchRecords = async () => {
  try {
    const response = await fetch('/api/traffic');
    if (!response.ok) throw new Error('Failed to fetch records');
    const data = await response.json();
    records.value = data.map((r: TrafficRecord) => ({ ...r, isExpanded: false }));
  } catch (err: any) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
};

const toggleExpand = (record: TrafficRecord) => {
  record.isExpanded = !record.isExpanded;
};

const toggleTimeSort = () => {
  timeSortOrder.value = timeSortOrder.value === 'desc' ? 'asc' : 'desc';
};

const parseHeaders = (headersStr: string): Record<string, string[]> => {
  try {
    return JSON.parse(headersStr);
  } catch {
    return {};
  }
};

const getHeaderValue = (headers: Record<string, string[]>, key: string): string => {
  const values = headers[key] || headers[key.toLowerCase()];
  return values ? values.join(', ') : '-';
};

const formatSize = (headersStr: string): string => {
  const headers = parseHeaders(headersStr);
  const contentLength = getHeaderValue(headers, 'Content-Length');
  if (contentLength === '-') return 'Unknown';
  const size = parseInt(contentLength, 10);
  if (isNaN(size)) return 'Unknown';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`;
  return `${(size / (1024 * 1024)).toFixed(2)} MB`;
};

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString();
};

const base64ToUint8Array = (base64: string): Uint8Array => {
  const binaryString = atob(base64);
  const bytes = new Uint8Array(binaryString.length);
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes;
};

const toggleReqBody = async (record: TrafficRecord) => {
  if (record.showReqBody) {
    record.showReqBody = false;
    return;
  }
  record.showReqBody = true;
  
  if (!record.req_body) {
    record.parsedReqBody = '';
    return;
  }

  if (record.parsedReqBody !== undefined) {
    return; // Already parsed
  }

  record.reqBodyLoading = true;
  try {
    const bytes = base64ToUint8Array(record.req_body);
    const headers = parseHeaders(record.req_headers);
    const contentType = getHeaderValue(headers, 'Content-Type').toLowerCase();
    
    // Attempt universal magic bytes detection
    const type = await fileTypeFromBuffer(bytes);
    let prefix = '';
    let isBinary = false;
    
    if (type) {
      prefix = `Detected Type: ${type.mime} (${type.ext})\n\n`;
      if (!type.mime.startsWith('text/') && !type.mime.includes('json') && !type.mime.includes('xml')) {
        isBinary = true;
      }
    } else if (!isUtf8(bytes)) {
      isBinary = true;
    }

    if (isBinary) {
      record.parsedReqBody = prefix + hexy(bytes, { format: 'twos' });
      return;
    }

    // It's mostly text, try decoding
    const decodedStr = new TextDecoder('utf-8').decode(bytes);

    if (contentType.includes('application/json')) {
      try {
        const jsonObj = JSON.parse(decodedStr);
        record.parsedReqBody = prefix + JSON.stringify(jsonObj, null, 2);
        return;
      } catch {
        // Fallback to plain text
      }
    } else if (contentType.includes('application/x-www-form-urlencoded')) {
      try {
        const params = new URLSearchParams(decodedStr);
        let result = '';
        for (const [key, value] of params.entries()) {
          result += `${key}: ${value}\n`;
        }
        if (result) {
          record.parsedReqBody = prefix + result;
          return;
        }
      } catch {
        // Fallback
      }
    }
    
    record.parsedReqBody = prefix + decodedStr;
  } catch (e: any) {
    record.parsedReqBody = 'Error parsing body: ' + e.message;
  } finally {
    record.reqBodyLoading = false;
  }
};

const copyToClipboard = async (text: string, e: Event) => {
  e.stopPropagation(); // Prevent expanding/collapsing row when clicking copy
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
    } catch (err) {
      console.error('Failed to copy text: ', err);
    }
  } else {
    // Fallback for non-secure contexts (e.g., http://192.168.x.x)
    const textArea = document.createElement("textarea");
    textArea.value = text;
    // Avoid scrolling to bottom
    textArea.style.top = "0";
    textArea.style.left = "0";
    textArea.style.position = "fixed";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
      const successful = document.execCommand('copy');
      if (!successful) {
        console.error('Fallback: Copying text command was unsuccessful');
      }
    } catch (err) {
      console.error('Fallback: Oops, unable to copy', err);
    }
    document.body.removeChild(textArea);
  }
};

// Computed Properties for Filtering & Sorting
const uniqueMethods = computed(() => {
  return Array.from(new Set(records.value.map(r => r.method))).sort();
});

const uniqueStatuses = computed(() => {
  return Array.from(new Set(records.value.map(r => String(r.status)))).sort();
});

const uniqueHosts = computed(() => {
  return Array.from(new Set(records.value.map(r => r.dst_host))).sort();
});

const uniqueUrls = computed(() => {
  return Array.from(new Set(records.value.map(r => r.url))).sort();
});

const filteredAndSortedRecords = computed(() => {
  let result = records.value.filter(record => {
    if (filterMethod.value && record.method !== filterMethod.value) return false;
    if (filterStatus.value && String(record.status) !== filterStatus.value) return false;
    if (filterHost.value && !record.dst_host.toLowerCase().includes(filterHost.value.toLowerCase())) return false;
    if (filterUrl.value && !record.url.toLowerCase().includes(filterUrl.value.toLowerCase())) return false;
    return true;
  });

  result.sort((a, b) => {
    const timeA = new Date(a.created_at).getTime();
    const timeB = new Date(b.created_at).getTime();
    return timeSortOrder.value === 'desc' ? timeB - timeA : timeA - timeB;
  });

  return result;
});

onMounted(fetchRecords);
</script>

<template>
  <div class="container">
    <header class="header">
      <h1>Traffic Monitor</h1>
      <div v-if="loading" class="status">Loading...</div>
      <div v-if="error" class="error">{{ error }}</div>
      <button @click="fetchRecords" class="refresh-btn">Refresh</button>
    </header>

    <main class="main">
      <div class="table-container">
        <table class="traffic-table">
          <thead>
            <tr>
              <th class="col-time sortable header-text empty-filter" @click="toggleTimeSort">
                <div class="flex-align-center">
                  Time 
                  <span class="sort-icon">
                    <svg v-if="timeSortOrder === 'desc'" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"></line><polyline points="19 12 12 19 5 12"></polyline></svg>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="19" x2="12" y2="5"></line><polyline points="5 12 12 5 19 12"></polyline></svg>
                  </span>
                </div>
              </th>
              <th class="col-method">
                <select v-model="filterMethod" class="header-control header-select" :class="filterMethod ? ['badge', 'method', filterMethod.toLowerCase()] : 'empty-filter'" @click.stop>
                  <option value="">Method</option>
                  <option v-for="m in uniqueMethods" :key="m" :value="m">{{ m }}</option>
                </select>
              </th>
              <th class="col-status">
                <select v-model="filterStatus" class="header-control header-select" :class="filterStatus ? ['badge', 'status', `s${Math.floor(Number(filterStatus) / 100)}xx`] : 'empty-filter'" @click.stop>
                  <option value="">Status</option>
                  <option v-for="s in uniqueStatuses" :key="s" :value="s">{{ s }}</option>
                </select>
              </th>
              <th class="col-host">
                <input type="text" v-model="filterHost" list="host-options" placeholder="Host" class="header-control header-input" :class="!filterHost ? 'empty-filter' : ''" @click.stop>
                <datalist id="host-options">
                  <option v-for="h in uniqueHosts" :key="h" :value="h"></option>
                </datalist>
              </th>
              <th class="col-url">
                <input type="text" v-model="filterUrl" list="url-options" placeholder="URL" class="header-control header-input url-header-input" :class="!filterUrl ? 'empty-filter' : ''" @click.stop>
                <datalist id="url-options">
                  <option v-for="u in uniqueUrls" :key="u" :value="u"></option>
                </datalist>
              </th>
              <th class="col-size header-text empty-filter">Size</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="record in filteredAndSortedRecords" :key="record.resp_body_path">
              <tr class="record-row" :class="{ expanded: record.isExpanded }" @click="toggleExpand(record)">
                <td>{{ formatDate(record.created_at) }}</td>
                <td><span class="badge method" :class="record.method.toLowerCase()">{{ record.method }}</span></td>
                <td><span class="badge status" :class="`s${Math.floor(record.status / 100)}xx`">{{ record.status }}</span></td>
                <td>
                  <div class="copy-wrapper">
                    <span class="copy-text truncate" :title="record.dst_host">{{ record.dst_host }}</span>
                    <button class="copy-btn" @click="(e) => copyToClipboard(record.dst_host, e)" title="Copy">
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                    </button>
                  </div>
                </td>
                <td>
                  <div class="copy-wrapper url-cell">
                    <span class="copy-text truncate" :title="record.url">{{ record.url }}</span>
                    <button class="copy-btn" @click="(e) => copyToClipboard(record.url, e)" title="Copy">
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                    </button>
                  </div>
                </td>
                <td>{{ formatSize(record.resp_headers) }}</td>
              </tr>
              <tr v-if="record.isExpanded" class="details-row">
                <td colspan="6">
                  <div class="details-content">
                    <div class="details-grid">
                      <section class="headers-section">
                        <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border-color); padding-bottom: 8px; margin-bottom: 12px;">
                          <h3 style="margin: 0; border: none; padding: 0;">Request Headers</h3>
                          <button v-if="record.req_body" @click="toggleReqBody(record)" class="view-body-btn">
                            {{ record.showReqBody ? 'Hide Request Body' : 'View Request Body' }}
                          </button>
                        </div>
                        <table class="headers-table">
                          <tr class="highlight-ip">
                            <td class="header-key">Request IP</td>
                            <td class="header-value">
                              <div class="copy-wrapper">
                                <span class="copy-text">{{ record.src_ip }}</span>
                                <button class="copy-btn" @click="(e) => copyToClipboard(record.src_ip, e)">
                                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                                </button>
                              </div>
                            </td>
                          </tr>
                          <tr v-for="(values, key) in parseHeaders(record.req_headers)" :key="key">
                            <td class="header-key">{{ key }}</td>
                            <td class="header-value">
                              <div class="copy-wrapper">
                                <span class="copy-text truncate-header" :title="values.join(', ')">{{ values.join(', ') }}</span>
                                <button class="copy-btn" @click="(e) => copyToClipboard(values.join(', '), e)">
                                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                                </button>
                              </div>
                            </td>
                          </tr>
                        </table>
                        
                        <div v-if="record.showReqBody" class="request-body-section">
                          <div v-if="record.reqBodyLoading">Parsing body...</div>
                          <pre v-else class="body-pre">{{ record.parsedReqBody }}</pre>
                        </div>
                      </section>
                      
                      <section class="headers-section">
                        <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border-color); padding-bottom: 8px; margin-bottom: 12px;">
                          <h3 style="margin: 0; border: none; padding: 0;">Response Headers</h3>
                          <a :href="`/responsebody/${record.resp_body_path}`" target="_blank" class="view-body-btn">View Response Body</a>
                        </div>
                        <table class="headers-table">
                          <tr class="highlight-ip">
                            <td class="header-key">Response IP</td>
                            <td class="header-value">
                              <div class="copy-wrapper">
                                <span class="copy-text">{{ record.dst_ip || 'Unknown' }}</span>
                                <button class="copy-btn" @click="(e) => copyToClipboard(record.dst_ip || 'Unknown', e)">
                                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                                </button>
                              </div>
                            </td>
                          </tr>
                          <tr v-for="(values, key) in parseHeaders(record.resp_headers)" :key="key">
                            <td class="header-key">{{ key }}</td>
                            <td class="header-value">
                              <div class="copy-wrapper">
                                <span class="copy-text truncate-header" :title="values.join(', ')">{{ values.join(', ') }}</span>
                                <button class="copy-btn" @click="(e) => copyToClipboard(values.join(', '), e)">
                                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                                </button>
                              </div>
                            </td>
                          </tr>
                        </table>
                      </section>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="filteredAndSortedRecords.length === 0 && !loading">
              <td colspan="6" class="no-records">No matching records found.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<style>
:root {
  --bg-color: #f8f9fa;
  --text-color: #212529;
  --border-color: #dee2e6;
  --primary-color: #0d6efd;
  --hover-color: #f1f3f5;
  --expanded-bg: #fff;
  
  /* Status Colors */
  --s2xx: #28a745;
  --s3xx: #ffc107;
  --s4xx: #dc3545;
  --s5xx: #6c757d;

  /* Method Colors */
  --get: #28a745;
  --post: #007bff;
  --put: #ffc107;
  --delete: #dc3545;
}

body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background-color: var(--bg-color);
  color: var(--text-color);
}

.container {
  max-width: 100%;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  background: white;
  padding: 15px 25px;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.05);
}

.header h1 {
  margin: 0;
  font-size: 1.5rem;
  color: #333;
}

.refresh-btn {
  padding: 8px 16px;
  background-color: var(--primary-color);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  transition: background 0.2s;
}

.refresh-btn:hover {
  background-color: #0b5ed7;
}

.table-container {
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0,0,0,0.05);
  overflow: visible;
}

.traffic-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

/* Header Cells Redesign */
.traffic-table th {
  background: #f1f3f5;
  text-align: left;
  padding: 10px 15px;
  font-weight: 600;
  border-bottom: 2px solid var(--border-color);
  vertical-align: middle;
}

.flex-align-center {
  display: flex;
  align-items: center;
}

.header-text {
  color: #6c757d;
}

.header-control {
  width: 100%;
  box-sizing: border-box;
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 0.95rem;
  font-weight: 600;
  outline: none;
  color: var(--text-color);
}

.header-select {
  cursor: pointer;
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
}

.header-input {
  padding: 2px 0;
  border-bottom: 1px solid transparent;
  transition: border-color 0.2s;
}

.header-input:focus {
  border-bottom-color: var(--primary-color);
  color: var(--text-color) !important;
}

.header-input::placeholder {
  color: #6c757d;
  font-weight: 600;
  opacity: 1;
}

.empty-filter {
  color: #6c757d !important;
}

.empty-filter.header-select {
  background-image: url("data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22292.4%22%20height%3D%22292.4%22%3E%3Cpath%20fill%3D%22%236c757d%22%20d%3D%22M287%2069.4a17.6%2017.6%200%200%200-13-5.4H18.4c-5%200-9.3%201.8-12.9%205.4A17.6%2017.6%200%200%200%200%2082.2c0%205%201.8%209.3%205.4%2012.9l128%20127.9c3.6%203.6%207.8%205.4%2012.8%205.4s9.2-1.8%2012.8-5.4L287%2095c3.5-3.5%205.4-7.8%205.4-12.8%200-5-1.9-9.2-5.5-12.8z%22%2F%3E%3C%2Fsvg%3E");
  background-repeat: no-repeat;
  background-position: right 0px top 50%;
  background-size: .65rem auto;
  padding-right: 1.2rem;
}

select.badge {
  padding: 4px 24px 4px 8px !important;
  font-size: 0.85rem;
  border-radius: 4px;
  display: inline-block;
  width: auto;
  color: white !important;
  background-image: url("data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22292.4%22%20height%3D%22292.4%22%3E%3Cpath%20fill%3D%22%23FFFFFF%22%20d%3D%22M287%2069.4a17.6%2017.6%200%200%200-13-5.4H18.4c-5%200-9.3%201.8-12.9%205.4A17.6%2017.6%200%200%200%200%2082.2c0%205%201.8%209.3%205.4%2012.9l128%20127.9c3.6%203.6%207.8%205.4%2012.8%205.4s9.2-1.8%2012.8-5.4L287%2095c3.5-3.5%205.4-7.8%205.4-12.8%200-5-1.9-9.2-5.5-12.8z%22%2F%3E%3C%2Fsvg%3E");
  background-repeat: no-repeat;
  background-position: right 6px top 50%;
  background-size: .65rem auto;
}

select.badge.s3xx {
   color: #212529 !important;
   background-image: url("data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22292.4%22%20height%3D%22292.4%22%3E%3Cpath%20fill%3D%22%23212529%22%20d%3D%22M287%2069.4a17.6%2017.6%200%200%200-13-5.4H18.4c-5%200-9.3%201.8-12.9%205.4A17.6%2017.6%200%200%200%200%2082.2c0%205%201.8%209.3%205.4%2012.9l128%20127.9c3.6%203.6%207.8%205.4%2012.8%205.4s9.2-1.8%2012.8-5.4L287%2095c3.5-3.5%205.4-7.8%205.4-12.8%200-5-1.9-9.2-5.5-12.8z%22%2F%3E%3C%2Fsvg%3E");
}

.url-header-input {
  color: var(--text-color);
}
.url-header-input:not(.empty-filter) {
  color: var(--text-color) !important;
}

.sortable {
  cursor: pointer;
  user-select: none;
}

.sortable:hover {
  background-color: #e2e6ea;
}

.sort-icon {
  margin-left: 5px;
  display: flex;
  align-items: center;
}

.traffic-table td {
  padding: 12px 15px;
  border-bottom: 1px solid var(--border-color);
}

.record-row {
  cursor: pointer;
  transition: background 0.1s;
}

.record-row:hover {
  background-color: var(--hover-color);
}

.record-row.expanded {
  background-color: #e9ecef;
  border-bottom: none;
}

.no-records {
  text-align: center;
  color: #6c757d;
  padding: 30px !important;
}

/* Column Widths */
.col-time { width: 180px; }
.col-method { width: 110px; }
.col-status { width: 100px; }
.col-host { width: 220px; }
.col-url { width: auto; }
.col-size { width: 100px; }

.truncate {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.url-cell .copy-text {
  color: var(--primary-color);
  font-family: monospace;
}

.badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.85rem;
  font-weight: bold;
  color: white;
  text-transform: uppercase;
  display: inline-block;
}

.method.get { background-color: var(--get); }
.method.post { background-color: var(--post); }
.method.put { background-color: var(--put); }
.method.delete { background-color: var(--delete); }

.status.s2xx { background-color: var(--s2xx); }
.status.s3xx { background-color: var(--s3xx); color: #212529; }
.status.s4xx { background-color: var(--s4xx); }
.status.s5xx { background-color: var(--s5xx); }

/* Details Section */
.details-row td {
  padding: 0;
  background: #fdfdfd;
}

.details-content {
  padding: 20px 30px;
  border-left: 4px solid var(--primary-color);
}

.details-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 30px;
}

.headers-section {
  min-width: 0;
}

.headers-section h3 {
  margin-top: 0;
  font-size: 1.1rem;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 8px;
  margin-bottom: 12px;
}

.headers-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.headers-table tr:nth-child(even) {
  background-color: #f8f9fa;
}

.headers-table td {
  padding: 6px 10px;
  font-size: 0.9rem;
  border-bottom: 1px solid #f1f1f1;
}

.request-body-section {
  margin-top: 20px;
}

.request-body-section h3 {
  margin-top: 20px;
}

.body-pre {
  background: #f8f9fa;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 12px;
  margin: 0;
  overflow-x: auto;
  max-height: 300px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 0.85rem;
  white-space: pre;
}

.header-key {
  font-weight: 600;
  width: 160px;
  color: #495057;
}

.header-value {
  font-family: monospace;
}

.highlight-ip td {
  font-weight: 700;
  color: #000;
  background-color: #eef2f5 !important;
}

.highlight-ip .header-value {
  font-weight: 700;
}

.truncate-header {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Copy Button Logic */
.copy-wrapper {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
}

.copy-text {
  flex: 1;
  min-width: 0;
}

.copy-btn {
  opacity: 0;
  margin-left: 8px;
  background: #fff;
  border: 1px solid #ced4da;
  border-radius: 4px;
  padding: 4px 6px;
  color: #6c757d;
  cursor: pointer;
  transition: opacity 0.2s, background 0.2s, color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.copy-btn:hover {
  background: #e9ecef;
  color: var(--primary-color);
}

.copy-btn svg {
  pointer-events: none;
}

.record-row:hover .copy-btn,
.headers-table tr:hover .copy-btn {
  opacity: 1;
}

.actions {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.view-body-btn {
  display: inline-block;
  padding: 10px 20px;
  background-color: #6c757d;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-weight: 500;
  border: none;
  font-size: 1rem;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.2s;
}

.view-body-btn:hover {
  background-color: #5a6268;
}

@media (max-width: 1200px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
