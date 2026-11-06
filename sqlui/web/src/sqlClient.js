import axios from 'axios';

const TIMEOUT = 30000;

function apiBase() {
  const b = (typeof window !== 'undefined' && window.__SQLUI__ && window.__SQLUI__.apiBase) || '/sqlui/api';
  return String(b).replace(/\/+$/, '');
}

function enc(v) {
  return encodeURIComponent(v == null ? '' : String(v));
}

function unwrap(resp) {
  const data = resp && resp.data;
  if (data && data.error) {
    throw new Error(data.error);
  }
  return data;
}

function http(method, path, body) {
  return axios({
    method,
    url: apiBase() + path,
    data: body,
    timeout: TIMEOUT,
    headers: body ? { 'Content-Type': 'application/json' } : {},
  }).then(unwrap);
}

export function getConnections() {
  return http('get', '/connections');
}

export function getDatabases(connID) {
  return http('get', `/sql/${enc(connID)}/databases`);
}

export function getTables(connID, database) {
  return http('get', `/sql/${enc(connID)}/tables?database=${enc(database)}`);
}

export function getColumns(connID, database, table) {
  return http('get', `/sql/${enc(connID)}/columns?database=${enc(database)}&table=${enc(table)}`);
}

export function getTableData(connID, database, table, limit) {
  return http('get', `/sql/${enc(connID)}/table-data?database=${enc(database)}&table=${enc(table)}&limit=${enc(limit || 1000)}`);
}

export function query(connID, sql) {
  return http('post', `/sql/${enc(connID)}/query`, { sql });
}
