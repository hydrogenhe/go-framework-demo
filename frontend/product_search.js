// 已应用的条件由列表、翻页和导出共同使用；表单编辑后点击查询才生效。
let productFilters = {}, exportingProducts = false;

function productSearchForm() {
  const field = (key, label, type = 'text', attrs = '', wide = false) =>
    `<div class="field" ${wide ? 'style="grid-column:span 2"' : ''}><label>${label}</label><input name="${key}" type="${type}" value="${esc(productFilters[key] ?? '')}" ${attrs}></div>`;
  return `<form onsubmit="applyProductFilters(event)" class="form-grid product-search">
    <div class="wide"><strong>筛选商品</strong></div>
    ${field('name', '名称关键词', 'text', 'placeholder="输入商品名称关键词"', true)}
    <div class="field"><label for="product-sort">排序</label><select id="product-sort" name="sort">${[['updated_desc','更新时间倒序'],['updated_asc','更新时间正序']].map(([value,label])=>`<option value="${value}" ${(productFilters.sort||'updated_desc')===value?'selected':''}>${label}</option>`).join('')}</select></div>
    ${field('price_min', '最低价格（元）', 'number', 'min="0" step="0.01"')}
    ${field('price_max', '最高价格（元）', 'number', 'min="0" step="0.01"')}
    ${field('created_from', '创建时间从', 'datetime-local')}
    ${field('created_to', '创建时间至', 'datetime-local')}
    ${field('stock_min', '最低库存', 'number', 'min="0" step="1"')}
    ${field('stock_max', '最高库存', 'number', 'min="0" step="1"')}
    <div class="wide" style="display:flex;gap:12px"><button class="primary" style="width:auto;padding:10px 20px" ${syncingProducts ? 'disabled' : ''}>查询</button><button type="button" class="secondary" onclick="resetProductFilters()" ${syncingProducts ? 'disabled' : ''}>重置</button></div>
  </form>`;
}

function filterPriceFen(raw) {
  if (!/^\d+(\.\d{1,2})?$/.test(raw)) throw new Error('价格请输入非负数，最多两位小数');
  const [whole, fraction = ''] = raw.split('.');
  const fen = Number(whole) * 100 + Number(fraction.padEnd(2, '0'));
  if (!Number.isSafeInteger(fen)) throw new Error('价格范围过大');
  return fen;
}

function buildProductQuery(paginated = true, filters = productFilters) {
  const params = new URLSearchParams();
  if (paginated) { params.set('page', productPage); params.set('page_size', productPageSize); }
  if (filters.sort !== undefined && filters.sort !== null && String(filters.sort).trim() !== '') {
    params.set('sort', String(filters.sort).trim());
  }
  for (const key of ['name', 'price_min', 'price_max', 'stock_min', 'stock_max', 'created_from', 'created_to']) {
    const raw = filters[key];
    if (raw === undefined || raw === null || String(raw).trim() === '') continue;
    let value = String(raw).trim();
    if (key.startsWith('price_')) value = String(filterPriceFen(value));
    if (key.startsWith('stock_')) {
      const n = Number(value);
      if (!Number.isInteger(n) || n < 0 || n > 2147483647) throw new Error('库存请输入有效的非负整数');
      value = String(n);
    }
    if (key.startsWith('created_')) {
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) throw new Error('创建时间无效');
      value = date.toISOString();
    }
    params.set(key, value);
  }
  for (const [lo, hi, label] of [['price_min', 'price_max', '价格'], ['stock_min', 'stock_max', '库存'], ['created_from', 'created_to', '创建时间']]) {
    if (params.has(lo) && params.has(hi)) {
      const a = params.get(lo), b = params.get(hi);
      if (lo.startsWith('created_') ? Date.parse(a) > Date.parse(b) : Number(a) > Number(b)) throw new Error(label + '下限不能大于上限');
    }
  }
  return params;
}

async function applyProductFilters(event) {
  event.preventDefault();
  if (syncingProducts) return;
  const filters = Object.fromEntries(new FormData(event.target));
  try { buildProductQuery(false, filters); } catch (err) { toast(err.message); return; }
  productFilters = filters;
  await changeProductPage(1);
}

async function resetProductFilters() {
  if (syncingProducts) return;
  productFilters = {};
  await changeProductPage(1);
}

async function exportProducts() {
  if (exportingProducts) return;
  exportingProducts = true;
  render();
  try {
    const response = await apiFetch('/api/products/export?' + buildProductQuery(false));
    if (!response.ok) {
      const error = await response.json().catch(() => null);
      throw new Error(error?.message || '导出失败');
    }
    const blob = await response.blob(), url = URL.createObjectURL(blob), link = document.createElement('a');
    link.href = url;
    link.download = 'products-' + new Date().toISOString().replace(/[:.]/g, '-') + '.xlsx';
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 60000);
    toast('已导出 ' + (response.headers.get('X-Export-Count') ?? '全部') + ' 个商品');
  } catch (err) { toast('商品导出失败：' + err.message); }
  finally { exportingProducts = false; render(); }
}
