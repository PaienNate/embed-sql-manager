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


