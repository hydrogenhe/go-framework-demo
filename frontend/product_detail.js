const detailContent = document.querySelector('#content');
const detailID = new URLSearchParams(location.search).get('id');

function escapeDetail(value) {
  return String(value ?? '').replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;'
  })[ch]);
}

function renderProductDetail(product) {
  const image = product.image
    ? `<img src="${escapeDetail(product.image)}" alt="${escapeDetail(product.name)}" onerror="this.replaceWith(document.createTextNode('图片暂不可用'))">`
    : '暂无商品图片';
  detailContent.className = 'card';
  detailContent.innerHTML = `
    <div class="visual">${image}</div>
    <section class="details">
      <div class="eyebrow">PRODUCT DETAILS</div>
      <h1>${escapeDetail(product.name)}</h1>
      <div class="price">¥${(product.price / 100).toFixed(2)}</div>
      <div class="meta"><span class="label">商品 ID</span><span>${escapeDetail(product.id)}</span></div>
      <div class="meta"><span class="label">库存</span><span>${product.stock}</span></div>
      <div class="meta"><span class="label">资料版本</span><span>${product.version}</span></div>
      <div class="meta"><span class="label">创建时间</span><span>${escapeDetail(new Date(product.create_time).toLocaleString('zh-CN'))}</span></div>
      <div class="meta"><span class="label">更新时间</span><span>${escapeDetail(new Date(product.update_time).toLocaleString('zh-CN'))}</span></div>
    </section>`;
}

async function loadProductDetail() {
  let session;
  try { session=JSON.parse(localStorage.getItem('demo-session-v2')||'null'); } catch (_) {}
  if(!session?.token){location.replace('index.html');return}
  if (!detailID || !/^[1-9]\d*$/.test(detailID)) {
    detailContent.textContent = '商品 ID 无效';
    return;
  }
  try {
    const response = await fetch(`${window.DEMO_API_BASE}/api/products/${encodeURIComponent(detailID)}/detail`, {
      method: 'PUT', headers: { Authorization: 'Bearer '+session.token, 'X-Log-ID': crypto.randomUUID?.() || `${Date.now()}` }
    });
    if(response.status===401){
      const current=JSON.parse(localStorage.getItem('demo-session-v2')||'null');
      if(current?.token===session.token)localStorage.removeItem('demo-session-v2');
      location.replace('index.html');return;
    }
    const body = await response.json().catch(() => null);
    if (!response.ok || body?.code !== 0) throw new Error(body?.message || '商品查询失败');
    renderProductDetail(body.data);
  } catch (error) {
    detailContent.className = 'state';
    detailContent.textContent = error.message || '商品查询失败，请稍后重试';
  }
}

loadProductDetail();
