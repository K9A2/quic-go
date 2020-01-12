(function(){function n(a){var b=0;return function(){return b<a.length?{done:!1,value:a[b++]}:{done:!0}}}
function p(a){var b="undefined"!=typeof Symbol&&Symbol.iterator&&a[Symbol.iterator];return b?b.call(a):{next:n(a)}}
function q(a){if(!(a instanceof Array)){a=p(a);for(var b,c=[];!(b=a.next()).done;)c.push(b.value);a=c}return a}
var r="function"==typeof Object.defineProperties?Object.defineProperty:function(a,b,c){a!=Array.prototype&&a!=Object.prototype&&(a[b]=c.value)},u="undefined"!=typeof window&&window===this?this:"undefined"!=typeof global&&null!=global?global:this;
function v(a,b){if(b){for(var c=u,g=a.split("."),h=0;h<g.length-1;h++){var k=g[h];k in c||(c[k]={});c=c[k]}g=g[g.length-1];h=c[g];k=b(h);k!=h&&null!=k&&r(c,g,{configurable:!0,writable:!0,value:k})}}
v("String.prototype.endsWith",function(a){return a?a:function(b,c){if(null==this)throw new TypeError("The 'this' value for String.prototype.endsWith must not be null or undefined");if(b instanceof RegExp)throw new TypeError("First argument to String.prototype.endsWith must not be a regular expression");var g=this+"";b+="";void 0===c&&(c=g.length);for(var h=Math.max(0,Math.min(c|0,g.length)),k=b.length;0<k&&0<h;)if(g[--h]!=b[--k])return!1;return 0>=k}});
function w(a,b){return Object.prototype.hasOwnProperty.call(a,b)}
function y(){y=function(){};
u.Symbol||(u.Symbol=z)}
function A(a,b){this.a=a;r(this,"description",{configurable:!0,writable:!0,value:b})}
A.prototype.toString=function(){return this.a};
var z=function(){function a(c){if(this instanceof a)throw new TypeError("Symbol is not a constructor");return new A("jscomp_symbol_"+(c||"")+"_"+b++,c)}
var b=0;return a}();
function B(){y();var a=u.Symbol.iterator;a||(a=u.Symbol.iterator=u.Symbol("Symbol.iterator"));"function"!=typeof Array.prototype[a]&&r(Array.prototype,a,{configurable:!0,writable:!0,value:function(){return C(n(this))}});
B=function(){}}
function C(a){B();a={next:a};a[u.Symbol.iterator]=function(){return this};
return a}
v("WeakMap",function(a){function b(d){this.a=(f+=Math.random()+1).toString();if(d){d=p(d);for(var e;!(e=d.next()).done;)e=e.value,this.set(e[0],e[1])}}
function c(){}
function g(d){var e=typeof d;return"object"===e&&null!==d||"function"===e}
function h(d){if(!w(d,m)){var e=new c;r(d,m,{value:e})}}
function k(d){var e=Object[d];e&&(Object[d]=function(l){if(l instanceof c)return l;h(l);return e(l)})}
if(function(){if(!a||!Object.seal)return!1;try{var d=Object.seal({}),e=Object.seal({}),l=new a([[d,2],[e,3]]);if(2!=l.get(d)||3!=l.get(e))return!1;l["delete"](d);l.set(e,4);return!l.has(d)&&4==l.get(e)}catch(t){return!1}}())return a;
var m="$jscomp_hidden_"+Math.random();k("freeze");k("preventExtensions");k("seal");var f=0;b.prototype.set=function(d,e){if(!g(d))throw Error("Invalid WeakMap key");h(d);if(!w(d,m))throw Error("WeakMap key fail: "+d);d[m][this.a]=e;return this};
b.prototype.get=function(d){return g(d)&&w(d,m)?d[m][this.a]:void 0};
b.prototype.has=function(d){return g(d)&&w(d,m)&&w(d[m],this.a)};
b.prototype["delete"]=function(d){return g(d)&&w(d,m)&&w(d[m],this.a)?delete d[m][this.a]:!1};
return b});
v("Map",function(a){function b(){var f={};return f.previous=f.next=f.head=f}
function c(f,d){var e=f.a;return C(function(){if(e){for(;e.head!=f.a;)e=e.previous;for(;e.next!=e.head;)return e=e.next,{done:!1,value:d(e)};e=null}return{done:!0,value:void 0}})}
function g(f,d){var e=d&&typeof d;"object"==e||"function"==e?k.has(d)?e=k.get(d):(e=""+ ++m,k.set(d,e)):e="p_"+d;var l=f.f[e];if(l&&w(f.f,e))for(var t=0;t<l.length;t++){var x=l[t];if(d!==d&&x.key!==x.key||d===x.key)return{id:e,list:l,index:t,b:x}}return{id:e,list:l,index:-1,b:void 0}}
function h(f){this.f={};this.a=b();this.size=0;if(f){f=p(f);for(var d;!(d=f.next()).done;)d=d.value,this.set(d[0],d[1])}}
if(function(){if(!a||"function"!=typeof a||!a.prototype.entries||"function"!=typeof Object.seal)return!1;try{var f=Object.seal({x:4}),d=new a(p([[f,"s"]]));if("s"!=d.get(f)||1!=d.size||d.get({x:4})||d.set({x:4},"t")!=d||2!=d.size)return!1;var e=d.entries(),l=e.next();if(l.done||l.value[0]!=f||"s"!=l.value[1])return!1;l=e.next();return l.done||4!=l.value[0].x||"t"!=l.value[1]||!e.next().done?!1:!0}catch(t){return!1}}())return a;
B();var k=new WeakMap;h.prototype.set=function(f,d){f=0===f?0:f;var e=g(this,f);e.list||(e.list=this.f[e.id]=[]);e.b?e.b.value=d:(e.b={next:this.a,previous:this.a.previous,head:this.a,key:f,value:d},e.list.push(e.b),this.a.previous.next=e.b,this.a.previous=e.b,this.size++);return this};
h.prototype["delete"]=function(f){f=g(this,f);return f.b&&f.list?(f.list.splice(f.index,1),f.list.length||delete this.f[f.id],f.b.previous.next=f.b.next,f.b.next.previous=f.b.previous,f.b.head=null,this.size--,!0):!1};
h.prototype.clear=function(){this.f={};this.a=this.a.previous=b();this.size=0};
h.prototype.has=function(f){return!!g(this,f).b};
h.prototype.get=function(f){return(f=g(this,f).b)&&f.value};
h.prototype.entries=function(){return c(this,function(f){return[f.key,f.value]})};
h.prototype.keys=function(){return c(this,function(f){return f.key})};
h.prototype.values=function(){return c(this,function(f){return f.value})};
h.prototype.forEach=function(f,d){for(var e=this.entries(),l;!(l=e.next()).done;)l=l.value,f.call(d,l[1],l[0],this)};
h.prototype[Symbol.iterator]=h.prototype.entries;var m=0;return h});
v("Set",function(a){function b(c){this.a=new Map;if(c){c=p(c);for(var g;!(g=c.next()).done;)this.add(g.value)}this.size=this.a.size}
if(function(){if(!a||"function"!=typeof a||!a.prototype.entries||"function"!=typeof Object.seal)return!1;try{var c=Object.seal({x:4}),g=new a(p([c]));if(!g.has(c)||1!=g.size||g.add(c)!=g||1!=g.size||g.add({x:4})!=g||2!=g.size)return!1;var h=g.entries(),k=h.next();if(k.done||k.value[0]!=c||k.value[1]!=c)return!1;k=h.next();return k.done||k.value[0]==c||4!=k.value[0].x||k.value[1]!=k.value[0]?!1:h.next().done}catch(m){return!1}}())return a;
B();b.prototype.add=function(c){c=0===c?0:c;this.a.set(c,c);this.size=this.a.size;return this};
b.prototype["delete"]=function(c){c=this.a["delete"](c);this.size=this.a.size;return c};
b.prototype.clear=function(){this.a.clear();this.size=0};
b.prototype.has=function(c){return this.a.has(c)};
b.prototype.entries=function(){return this.a.entries()};
b.prototype.values=function(){return this.a.values()};
b.prototype.keys=b.prototype.values;b.prototype[Symbol.iterator]=b.prototype.values;b.prototype.forEach=function(c,g){var h=this;this.a.forEach(function(k){return c.call(g,k,k,h)})};
return b});
var D=this||self;function E(a,b){var c=a.split("."),g=D;c[0]in g||"undefined"==typeof g.execScript||g.execScript("var "+c[0]);for(var h;c.length&&(h=c.shift());)c.length||void 0===b?g[h]&&g[h]!==Object.prototype[h]?g=g[h]:g=g[h]={}:g[h]=b}
;/*

 Copyright The Closure Library Authors.
 SPDX-License-Identifier: Apache-2.0
*/
var F=/^(?:([^:/?#.]+):)?(?:\/\/(?:([^/?#]*)@)?([^/#?]*?)(?::([0-9]+))?(?=[/#?]|$))?([^?#]+)?(?:\?([^#]*))?(?:#([\s\S]*))?$/;function G(a){return a?decodeURI(a):a}
;var H=window.yt&&window.yt.config_||window.ytcfg&&window.ytcfg.data_||{};E("yt.config_",H);function I(){var a=[];return"CSS_CLASS_WHITELIST"in H?H.CSS_CLASS_WHITELIST:a}
;var J=Object.freeze(["js-httpswwwgoogleanalyticscomanalyticsjs","js-chromeextensionpkedcjkdefgpdelpbcmbmeomcjbeemfm","video-","js-http","css-http"]),K=Object.freeze("document.appendChild document.body.appendChild document.querySelector document.querySelectorAll history.back history.go".split(" ")),L=Object.freeze("fonts.googleapis.com s0.2mdn.net securepubads.g.doubleclick.net ssl.google-analytics.com static.doubleclick.net www.google-analytics.com www.googletagservices.com www.youtube.com youtube.com".split(" ")),
M=Object.freeze(["pkedcjkdefgpdelpbcmbmeomcjbeemfm","fjhoaacokmgbjemoflkofnenfaiekifl","enhhojjnijigcajfphajepfemndkmdlo"]),N=Object.freeze(".corp.google.com .googlevideo.com .ytimg.com .google.com .googlesyndication.com .gstatic.com .prod.google.com .google.ru".split(" ")),O=Object.freeze(["chrome-extension","safari-extension","safari-resource","opera"]);function P(){return K.map(function(a){return Q(a)}).filter(function(a){return!!a})}
function Q(a){var b=a.split(".");a=b[b.length-1];b=b.reduce(function(c,g){return c&&c[g]},window);
if(!b)return a+" is missing";b=Function.prototype.toString.call(b).replace(/\n/g," ").replace(/  +/g," ");return b!="function "+a+"() { [native code] }"?a+" is not native, prologue: "+b.slice(0,50):null}
function R(a){var b=a.match(F)[1]||null;return O.some(function(c){return b==c})}
function S(a){var b=G(a.match(F)[3]||null);return!b||R(a)?!0:L.some(function(c){return b==c})||N.some(function(c){return b.endsWith(c)})}
function T(a){if(!R(a))return null;var b=G(a.match(F)[3]||null);return b?M.some(function(c){return b==c})?null:b:null}
function U(){var a=new Set;[].concat(q(document.querySelectorAll("script"))).forEach(function(b){b.src&&!S(b.src)&&a.add(b.src)});
[].concat(q(document.querySelectorAll("link[href]"))).forEach(function(b){"alternate"==b.rel||S(b.href)||a.add(b.href)});
return[].concat(q(a)).sort()}
function V(){var a=new Set;[].concat(q(document.querySelectorAll("script"))).forEach(function(b){b.src&&(b=T(b.src))&&a.add(b)});
return[].concat(q(a)).sort()}
function W(){var a=new Set;[].concat(q(window.document.querySelectorAll("*"))).forEach(function(b){[].concat(q(b.classList)).forEach(function(c){X()||a.add(c)})});
return a}
function X(){return J.some(function(){})}
function Y(){var a=new Set(I());if(!a.size)return[];var b=new Set;W().forEach(function(h){a.has(h)||X()||b.add(h)});
var c=[].concat(q(b)).sort();c=c.slice(0,15);var g=b.size-15;0<g&&c.push("...and "+g+" more.");return c}
;E("ytbin.polymer.shared.lib.tampering.info",function(){var a=Y(),b=U(),c=P(),g=V(),h=[];g.length&&h.push("extensions",g);a.length&&h.push("suspiciousClasses",a);b.length&&h.push("suspiciousIncludes",b);c.length&&h.push("suspiciousApis",c);return h.length?h:null});}).call(this);
