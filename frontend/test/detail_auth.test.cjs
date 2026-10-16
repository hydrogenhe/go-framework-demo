const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.resolve(__dirname,'../product_detail.js'),'utf8');
async function load(session,status=200){
 const content={textContent:'',innerHTML:'',className:''},storage=new Map(),requests=[],redirects=[];
 if(session)storage.set('demo-session-v2',JSON.stringify(session));
 const context=vm.createContext({URLSearchParams,location:{search:'?id=7',replace:url=>redirects.push(url)},window:{DEMO_API_BASE:'http://localhost:8080'},crypto:{randomUUID:()=> 'log-id'},document:{querySelector:()=>content},localStorage:{getItem:key=>storage.get(key),removeItem:key=>storage.delete(key)},fetch:async(url,options)=>{requests.push({url,options});return {status,ok:status===200,json:async()=>({code:0,data:{id:'7',name:'商品',price:100,stock:5,version:1}})}}});
 vm.runInContext(source,context);await new Promise(resolve=>setImmediate(resolve));
 return {content,storage,requests,redirects};
}
test('详情无会话直接返回登录页，不请求商品',async()=>{
 const app=await load(null);assert.deepEqual(app.redirects,['index.html']);assert.equal(app.requests.length,0);
});
test('详情携带凭证，过期时清理登录并跳转',async()=>{
 const app=await load({token:'expired'},401);assert.equal(app.requests[0].options.headers.Authorization,'Bearer expired');assert.equal(app.storage.has('demo-session-v2'),false);assert.deepEqual(app.redirects,['index.html']);assert.equal(app.content.innerHTML,'');
});
test('详情有效会话才展示商品',async()=>{
 const app=await load({token:'valid'});assert.match(app.content.innerHTML,/商品/);assert.equal(app.redirects.length,0);
});
