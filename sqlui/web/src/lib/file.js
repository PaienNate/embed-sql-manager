// Small file helpers shared by the query-tab and result-grid import/export
// actions. Kept IE11-friendly (no String.includes / Array.includes / spread).

export function downloadText(text, filename, mime) {
  const blob = new Blob([text], { type: mime || 'text/plain;charset=utf-8' });
  if (window.navigator && window.navigator.msSaveBlob) {
    window.navigator.msSaveBlob(blob, filename);
    return;
  }
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.style.display = 'none';
  document.body.appendChild(a);
  a.click();
  setTimeout(() => {
    if (a.parentNode) {
      a.parentNode.removeChild(a);
    }
    URL.revokeObjectURL(url);
  }, 0);
}

export function readFileText(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(reader.error || new Error('读取文件失败'));
    reader.readAsText(file, 'UTF-8');
  });
}

export function toCSV(columns, rows) {
  function esc(v) {
    if (v === null || v === undefined) {
      return '';
    }
    const s = String(v);
    if (s.indexOf('"') >= 0 || s.indexOf(',') >= 0 || s.indexOf('\n') >= 0 || s.indexOf('\r') >= 0) {
      return '"' + s.replace(/"/g, '""') + '"';
    }
    return s;
  }
  const lines = [columns.map(esc).join(',')];
  rows.forEach((row) => {
    lines.push(row.map(esc).join(','));
  });
  return lines.join('\r\n');
}

export function toJSON(columns, rows) {
  const objects = rows.map((row) => {
    const o = {};
    columns.forEach((name, i) => {
      o[name] = row[i] === undefined ? null : row[i];
    });
    return o;
  });
  return JSON.stringify(objects, null, 2);
}

// parseCSV recognizes quoted fields, escaped quotes and embedded newlines.
export function parseCSV(text) {
  const records = [];
  let row = [];
  let field = '';
  let inQuotes = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text.charAt(i);
    if (inQuotes) {
      if (ch === '"') {
        if (text.charAt(i + 1) === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
    } else if (ch === '"') {
      inQuotes = true;
    } else if (ch === ',') {
      row.push(field);
      field = '';
    } else if (ch === '\n') {
      row.push(field);
      field = '';
      records.push(row);
      row = [];
    } else if (ch !== '\r') {
      field += ch;
    }
  }
  if (field !== '' || row.length) {
    row.push(field);
    records.push(row);
  }
  if (records.length && records[records.length - 1].length === 1 && records[records.length - 1][0] === '') {
    records.pop();
  }
  if (!records.length) {
    return { columns: [], rows: [] };
  }
  const header = records.shift().map((name) => ({ name: String(name), type: '' }));
  return { columns: header, rows: records };
}

export function parseJSONResult(text) {
  const obj = JSON.parse(text);
  if (obj && Array.isArray(obj.columns) && Array.isArray(obj.rows)) {
    const columns = obj.columns.map((c) => (typeof c === 'string'
      ? { name: c, type: '' }
      : { name: c.name, type: c.type || '' }));
    return { columns, rows: obj.rows };
  }
  if (Array.isArray(obj)) {
    if (!obj.length) {
      return { columns: [], rows: [] };
    }
    if (Array.isArray(obj[0])) {
      let width = 0;
      obj.forEach((r) => { if (r.length > width) width = r.length; });
      const columns = [];
      for (let i = 0; i < width; i++) {
        columns.push({ name: 'col' + (i + 1), type: '' });
      }
      return { columns, rows: obj };
    }
    const keys = [];
    obj.forEach((o) => {
      Object.keys(o || {}).forEach((k) => {
        if (keys.indexOf(k) < 0) {
          keys.push(k);
        }
      });
    });
    const rows = obj.map((o) => keys.map((k) => (o && o[k] !== undefined ? o[k] : null)));
    return { columns: keys.map((k) => ({ name: k, type: '' })), rows };
  }
  throw new Error('JSON 格式不支持：需要 {columns, rows} 或对象数组');
}
