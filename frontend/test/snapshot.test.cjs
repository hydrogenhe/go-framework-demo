const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.resolve(__dirname,'../order_snapshot.js'),'utf8');
async function load(session,status=200){
 const content={textContent:'',innerHTML:'',className:''},storage=new Map(),requests=[],redirects=[];
 if(session)storage.set('demo-session-v2',JSON.stringify(session));
 const context=vm.createContext({URLSearchParams,location:{search:'?order_id=123&item_id=456',replace:url=>redirects.push(url)},window:{DEMO_API_BASE:'http://localhost:8080'},crypto:{randomUUID:()=> 'log-id'},document:{querySelector:()=>content},localStorage:{getItem:key=>storage.get(key),removeItem:key=>storage.delete(key)},fetch:async(url,options)=>{requests.push({url,options});return {status,ok:status===200,json:async()=>({code:0,data:{id:'456',order_id:'123',product_id:'7',product_name:'下单时的商品',image:'https://example.com/snapshot.png',unit_price_fen:1100,quantity:2,product_version:3}})}}});
 vm.runInContext(source,context);await new Promise(resolve=>setImmediate(resolve));
 return {content,storage,requests,redirects};
}
test('快照无会话直接返回登录页，不请求商品',async()=>{
 const app=await load(null);assert.deepEqual(app.redirects,['index.html']);assert.equal(app.requests.length,0);
});
test('快照携带凭证，过期时清理登录并跳转',async()=>{
 const app=await load({token:'expired'},401);assert.equal(app.requests[0].options.headers.Authorization,'Bearer expired');assert.equal(app.storage.has('demo-session-v2'),false);assert.deepEqual(app.redirects,['index.html']);assert.equal(app.content.innerHTML,'');
});
test('快照有效会话才展示商品',async()=>{
 const app=await load({token:'valid'});assert.match(app.content.innerHTML,/下单时的商品/);assert.match(app.content.innerHTML,/11.00/);assert.match(app.content.innerHTML,/22.00/);assert.match(app.requests[0].url,/orders\/123\/items\/456\/snapshot$/);assert.doesNotMatch(app.content.innerHTML,/库存/);assert.equal(app.redirects.length,0);
});
