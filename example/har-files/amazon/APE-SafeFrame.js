!function i(r,s,d){function c(t,e){if(!s[t]){if(!r[t]){var a="function"==typeof require&&require;if(!e&&a)return a(t,!0);if(l)return l(t,!0);var n=new Error("Cannot find module '"+t+"'");throw n.code="MODULE_NOT_FOUND",n}var o=s[t]={exports:{}};r[t][0].call(o.exports,function(e){return c(r[t][1][e]||e)},o,o.exports,i,r,s,d)}return s[t].exports}for(var l="function"==typeof require&&require,e=0;e<d.length;e++)c(d[e]);return c}({1:[function(e,t,a){var n=e("./lib/replace"),o=e("./lib/getSlotPlaceholder");t.exports.replace=n,t.exports.getSlotPlaceholder=o},{"./lib/getSlotPlaceholder":2,"./lib/replace":4}],2:[function(e,t,a){var s=e("./globals"),d="data-val";t.exports=function(e,t,a){var n=s.getDocument(),o=[e,a,t].join("_"),i=n.getElementById("ape_"+o+"_placement_ClickTracking");if(!(i&&i.hasAttribute&&"function"==typeof i.hasAttribute&&i.hasAttribute(d)&&i.getAttribute&&"function"==typeof i.getAttribute))return"";var r=i.getAttribute(d);return"string"!=typeof r?"":r}},{"./globals":3}],3:[function(e,t,a){t.exports.getDocument=function(){return document}},{}],4:[function(e,t,a){var o="&pd_rd_plhdr=t",i=/(&amp;|\?){1}pd_rd_plhdr=t(&amp;|'|&quot;){1}/g,r=/(&|\?){1}pd_rd_plhdr=t(&|'|"|\\"|\\'){1}/g;t.exports=function(e,t){var a=t,n=e;"string"!=typeof e||e===o?n="":(n.startsWith("&")&&(n=n.replace(/^&+/,"")),n.endsWith("&")&&(n=n.replace(/&+$/,"")));try{return""===n?a.replace(new RegExp("\\bpd_rd_plhdr=t&"),"").replace(new RegExp(o+"\\b"),"").replace(new RegExp("\\?pd_rd_plhdr=t\\b"),""):a.replace(r,"$1"+n+"$2").replace(i,"$1"+n.replace(/&/g,"&amp;")+"$2")}catch(e){return t}}},{}],5:[function(e,t,a){var n=e("./ajaxRequest");t.exports.Tracer=function(e,t){return this.traceId=e,this.adStartTime=t,this.storedTrace={},this.logTrace=function(e,t){if(void 0!==this.traceId){var a,n=(new Date).getTime();this.storedTrace.hasOwnProperty(e)||(this.storedTrace[e]=[]),(a=t===Object(t)?Object.assign&&"function"==typeof Object.assign?Object.assign({},t):JSON.parse(JSON.stringify(t)):(a='{ "'+e+'":"'+t+'"}',JSON.parse(a))).timeStamp=n,a.timeSinceAdStart=n-this.adStartTime,this.storedTrace[e].push(a)}},this.sendTrace=function(){var t=function(){console.log("failed to send request to /gp/adbarplus")};if(!function(e){for(var t in e)if(e.hasOwnProperty(t))return!1;return!0}(this.storedTrace)){var e="/gp/adbarplus?traceId="+this.traceId+"&systemName=browser";for(var a in n.sendAjaxRequest(e,"POST",JSON.stringify(this.storedTrace),{"Content-Type":"application/x-www-form-urlencoded"},function(e){4===e.readyState&&200!==e.status&&t()},t),this.storedTrace)this.storedTrace.hasOwnProperty(a)&&delete this.storedTrace[a]}},this.bindSendTraceToPageOnLoad=function(){var e=function(e,t){return function(){return e.apply(t)}},t=function(){this.sendTrace()},a=function(){this.sendTrace(),window.setInterval(e(t,this),3e3)};"loading"!==document.readyState?e(a,this)():window.addEventListener?window.addEventListener("load",e(a,this)):document.attachEvent("onreadystatechange",function(){"complete"===document.readyState&&e(a,this)()})},void 0!==e&&this.bindSendTraceToPageOnLoad(),{traceId:this.traceId,adStartTime:this.adStartTime,storedTrace:this.storedTrace,allData:this.allData,logTrace:this.logTrace,sendTrace:this.sendTrace}}},{"./ajaxRequest":6}],6:[function(e,t,a){t.exports.sendAjaxRequest=function(e,t,a,n,o,i){try{var r=null;if(window.XMLHttpRequest?r=new XMLHttpRequest:i(),r){if(r.onreadystatechange=function(){o(r)},r.open(t,e,!0),null!==n)for(var s in n)r.setRequestHeader(s,n[s]);r.send(a)}else i()}catch(e){i()}}},{}],7:[function(e,t,a){var l=e("../host/metrics/counters");t.exports.checkCache=function(e,t,a,n,o){var i=l.CACHE_COUNTERS;if("undefined"!=typeof performance&&void 0!==performance.getEntriesByType){var r=performance.getEntriesByType("resource");if(void 0!==r&&Array.isArray(r)&&!(r.length<1)&&void 0!==r[0].duration){var s=void 0!==r[0].transferSize?function(e,t){0===e.transferSize?d(t+"cached"):d(t+"uncached")}:function(e,t){e.duration<20?d(t+"fastload"):d(t+"slowload")};c(e,i.SF_LIBRARY),c(t,i.SF_HTML)}}function d(e){o(a,n,e,1)}function c(e,t){if(e)for(var a=0;a<r.length;a++){var n=r[a];if(n.name&&-1!==n.name.indexOf(e))return void s(n,t)}}}},{"../host/metrics/counters":14}],8:[function(e,t,a){var n=e("@apejs/click-tracking");t.exports.getSlotPlaceholder=function(e){if(!("pageType"in e&&"subPageType"in e&&"slotName"in e))return"";try{return n.getSlotPlaceholder(e.pageType,e.subPageType,e.slotName)}catch(e){return""}}},{"@apejs/click-tracking":1}],9:[function(e,u,t){
/*
    @license
    Underscore.js 1.8.3
    http://underscorejs.org
    (c) 2009-2015 Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
    Underscore may be freely distributed under the MIT license.
*/
var n=function(){return window.P&&window.P.AUI_BUILD_DATE};u.exports.throttle=function(a,n,o){var i,r,s,d=null,c=0;o||(o={});var l=function(){c=!1===o.leading?0:u.exports.now(),d=null,s=a.apply(i,r),d||(i=r=null)};return function(){var e=u.exports.now();c||!1!==o.leading||(c=e);var t=n-(e-c);return i=this,r=arguments,t<=0||n<t?(d&&(clearTimeout(d),d=null),c=e,s=a.apply(i,r),d||(i=r=null)):d||!1===o.trailing||(d=setTimeout(l,t)),s}},u.exports.now=function(){return Date.now?Date.now():(new Date).getTime()},u.exports.addListener=function(e,t,a){e.addEventListener?e.addEventListener(t,a,!1):window.attachEvent&&e.attachEvent("on"+t,a)},u.exports.addWindowListener=function(e,t){u.exports.addListener(window,e,t)},u.exports.removeWindowListener=function(e,t){window.removeEventListener?window.removeEventListener(e,t,!1):window.detachEvent&&window.detachEvent("on"+e,t)},u.exports.ensureMessageListener=function(e){u.exports.removeWindowListener("message",e),u.exports.addWindowListener("message",e)},u.exports.decodeBase64=function(e){var t,a,n,o,i,r,s="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",d="",c=0;for(e=e.replace(/[^A-Za-z0-9\+\/\=]/g,"");c<e.length;)t=s.indexOf(e.charAt(c++))<<2|(o=s.indexOf(e.charAt(c++)))>>4,a=(15&o)<<4|(i=s.indexOf(e.charAt(c++)))>>2,n=(3&i)<<6|(r=s.indexOf(e.charAt(c++))),d+=String.fromCharCode(t),64!=i&&(d+=String.fromCharCode(a)),64!=r&&(d+=String.fromCharCode(n));return d=function(e){for(var t="",a=0,n=0,o=0,i=0;a<e.length;)(n=e.charCodeAt(a))<128?(t+=String.fromCharCode(n),a++):191<n&&n<224?(o=e.charCodeAt(a+1),t+=String.fromCharCode((31&n)<<6|63&o),a+=2):(o=e.charCodeAt(a+1),i=e.charCodeAt(a+2),t+=String.fromCharCode((15&n)<<12|(63&o)<<6|63&i),a+=3);return t}(d)},u.exports.createScript=function(e,t,a,n,o){if(!document.getElementById(a)){var i=document.createElement("script");return i.async=!0,i.setAttribute("crossorigin","anonymous"),i.src=e,i.type=t,i.id=a,i.onerror=n,i.onload=o,i}},u.exports.isAUIAvailable=n,u.exports.safeFunctionWrapper=function(e,t,a){return n()&&"function"==typeof window.P.guardError?P.guardError("APE-SafeFrame",e):function(){try{e.apply(this,arguments)}catch(e){"function"==typeof t&&a&&t(a,e)}}},u.exports.getCookie=function(e){var t=e+"=";try{for(var a=decodeURIComponent(document.cookie).split(";"),n=0;n<a.length;n++){for(var o=a[n];" "==o.charAt(0);)o=o.substring(1);if(0==o.indexOf(t))return o.substring(t.length,o.length)}}catch(e){}return""},u.exports.disableCookieAccess=function(){try{Object&&Object.defineProperty&&"function"==typeof Object.defineProperty?Object.defineProperty(document,"cookie",{get:function(){return""},set:function(){}}):(document.__defineGetter__("cookie",function(){return""}),document.__defineSetter__("cookie",function(){}))}catch(e){}},u.exports.setObjectStyles=function(e,t){if(e&&t)for(var a in t)e.style[a]=t[a];return e},u.exports.ABP_STATUS={1:"Enabled",0:"NotEnabled","-1":"Unknown"}},{}],10:[function(g,e,t){
/**
 * @license
 * Copyright (c) 2014, Amazon.com
 * APE SafeFrame v1.50.b3059ee -- 2019-12-19T22:53:11+0000
*/
!function(k,L){var e=g("./messenger/msgHandler"),t=g("./metrics/counters"),r=g("../components/cacheChecker"),R=g("../components/adBarTracer"),n=g("./components/adFeedback"),a=g("./metrics/csm"),D=g("../components/clickTrackingHelper"),s=e.util,N=e.messenger,O=e.logError,d=N.appendErrorDetails,o=e.loadScript,F=a.sendCsmLatencyMetric,_=a.sendCsmCounter,P=a.addCsmTag,c=e.fireViewableLatencyMetrics,W=e.hasClass,l=e.createIframeWithAttributes,z=e.logCounter,B=e.collapseSlot,u=e.resizeSafeFrameAd,H=e.delayLoad,p=e.getMediaCentralOrigin,f=e.scriptValidator,m=e.sizeValidator,V=e.appendJsScript,U=e.checkAgainstWhitelist,j=s.ABP_STATUS;function i(){if(k.DAsf)k.DAsf.loadAds();else{k.DAsf={version:"1.50.b3059ee"},_(null,null,t.SF_VERSION_COUNTERS.VERSION+":"+k.DAsf.version,1);var g="text/x-dacx-safeframe",e=p(),h=e+"/sf-1.50.b3059ee._V445229579_.html",w=e+"/images/G/28/ape/sf/whitelisted/desktop/sf-1.50.b3059ee._V445229578_.html",v="data-arid",y="d16g_postMessageUnsupported",b="d16g_postMessageSupported",S=t.ABP_STATUS_COUNTERS,T=t.AD_LOAD_COUNTERS,a=t.MESSENGER_COUNTERS,o={},x={},A={},i=null;N.supportedCommands={sendAdBarTrace:function(e,t){e.options.arid in A&&A[e.options.arid].logTrace(t.field,t.traceInfo)},logAPIInvocation:function(e,t){_(null,null,a.API+t.apiName,1),_(e.options.slot,e.options.placementId,a.API+t.apiName,1),e.options.arid in A&&A[e.options.arid].logTrace("apiCalls",t)},resizeSafeFrameAd:function(e,t){s.addWindowListener("resize",o[e.options.arid].defaultResizeSafeFrameHandler),u(e.options.arid,e.options.size.width,e.options.size.height,e.options.maxAdWidth,e.options.adCreativeMetaData.adProgramId,e.options.programIdsToCollapse,e.options.minWidthToPunt,N,x)},changeSize:function(e,t){var a=e.options.allowedSizes;if(U(t,a,m))e.slot.style.width=t.width,e.iframe.height=t.height,e.iframe.width=t.width;else{var n="Size is not whitelisted: "+t.width+" x "+t.height+d(e.options.arid);O(n)}},collapseSlot:function(e,t){B(x[e.options.arid].placementDivId),"nav-sitewide-msg"===e.options.slotName&&H("amznJQ.available:navbarJSLoaded",function(){void 0!==parent.navbar&&"function"==typeof parent.navbar.unHideSWM&&parent.navbar.unHideSWM()})},embedScript:function(e,t){var a=e.options.allowedDomains;if(U(t.src,a,f))e.slot=L.getElementById(x[e.options.arid].placementDivId),void 0!==e.slot&&V(t.src,e.slot,t.charset);else{var n="Domain is not whitelisted: "+t.src+d(e.options.arid);O(n)}},logError:function(e,t){O(t.message+d(e.options.arid)+": "+e.options.slot,t.error)},sendCsmLatencyMetric:function(e,t){F(t.metric,e.options.slot,e.options.placementId,t.metricMsg,t.timestamp)},countMetric:function(e,t){t.isGlobal?_(null,null,t.metricMsg,t.value):_(e.options.slot,e.options.placementId,t.metricMsg,t.value)},addCsmTag:function(e,t){P(t.tag,e.options.slot,e.options.placementId,t.msg)},fireViewableLatencyMetrics:function(e,t){c(e.options.arid,e.options.slot,e.options.placementId,t.adLoadedTimestamp)},customMessage:function(e,t){N.customMessage(t.key,t.body)},enableViewabilityTracker:function(e){N.updateViewability(e.options.arid);var t=s.throttle(N.updateViewability,20);E(e.options.arid,e.options.slot,"viewabilityTracker",function(){t(e.options.arid)}),s.addWindowListener("scroll",o[e.options.arid].viewabilityTracker),s.addWindowListener("resize",o[e.options.arid].viewabilityTracker),s.addListener(L,"visibilitychange",o[e.options.arid].viewabilityTracker)},enableNoInventoryViewabilityTrackerAndInvokeFallback:function(e){N.takeSnapshotOfSlotPosition(e.options.arid),N.updateNoInventoryViewability(e.options.arid),N.sendMessageToAd(e.options.arid,"handleFallbackBehavior",{});var t=s.throttle(N.updateNoInventoryViewability,20);E(e.options.arid,e.options.slot,"noInventoryViewabilityTracker",function(){t(e.options.arid)}),s.addWindowListener("scroll",o[e.options.arid].noInventoryViewabilityTracker),s.addWindowListener("resize",o[e.options.arid].noInventoryViewabilityTracker),s.addListener(L,"visibilitychange",o[e.options.arid].noInventoryViewabilityTracker)},loadAdFeedback:function(e,t){var a=N.adMap[e.options.arid].iframe;e.options.adCreativeMetaData=t,n.appendAdFeedbackLinkToIframe(a,e.options,A)},safeFrameReady:function(e){},requestVideoAutoplay:function(e,t){if(i===e.options.arid&&N.sendCustomMessageToAd(e.options.arid,"videoAutoplayResponse",!0),null===i&&null!==e.options.arid){var a=L.getElementsByTagName("video"),n=a&&0===a.length;i=n?e.options.arid:null,N.sendCustomMessageToAd(e.options.arid,"videoAutoplayResponse",n)}},releaseVideoAutoplay:function(e,t){i=null,N.sendCustomMessageToAd(e.options.arid,"videoAutoplayReleased")},logSafeframeResourceTimingData:function(e,t){k.logSafeframeResourceTimingData(e.options,t)}},s.addWindowListener("message",N.receiveMessage),k.DAsf.registerCustomMessageListener=N.registerCustomMessageListener,k.DAsf.sendCustomMessage=N.sendCustomMessage,k.DAsf.loadAds=function(){var e,t,a=0,n=null,o=[];if("function"!=typeof L.getElementsByClassName){var i=L.getElementsByTagName("div"),r=L.getElementsByTagName("script"),s=0;for(s=0;s<i.length;s++)o[s]=i[s];for(s=0;s<r.length;s++)o[s+i.length]=r[s]}else o=L.getElementsByClassName(g);for(0===o.length&&(o=L.getElementsByTagName("script"));n=o[a++];)if("DIV"===n.tagName&&W(n,g)||n.getAttribute("type")===g){var d=n.getAttribute("data-ad-details")||n.text||n.innerHTML||n.innerText;try{var c="ape_"+(d=JSON.parse(d)).slot+"_placement",l=L.getElementById(c);if(!N.adMap[d.arid]&&l&&l.innerHTML&&(l.innerHTML="",n.removeAttribute(v)),n.getAttribute(v))continue;d.arid=d.arid||Math.random().toString(16).slice(2),A[d.arid]=new R.Tracer(d.traceId,k[d.slotName]&&k[d.slotName].adStartTime||0),A[d.arid].logTrace("safeFrameInput",d);var u={};u.caches=k.caches?k.caches:null,u.plugins=L.plugins?L.plugins:null,u.cookies=L.cookie?L.cookie:null,u.userAgents=navigator.userAgent?navigator.userAgent:null,A[d.arid].logTrace("browserData",u),n.setAttribute(v,d.arid),d.hostDomain=location.protocol+"//"+location.host,d.allowedSizes="object"==typeof d.allowedSizes&&0<=d.allowedSizes.length?d.allowedSizes.concat(d.size):[d.size];var p="d3l3lkinz3f56t.cloudfront.net,g-ecx.images-amazon.com,z-ecx.images-amazon.com,images-na.ssl-images-amazon.com,g-ec4.images-amazon.com,images-cn.ssl-images-amazon.com".split(",");if(d.allowedDomains="object"==typeof d.allowedDomains&&0<=d.allowedDomains.length?d.allowedDomains.concat(p):p,d.queryParams=C(),d.aPageStart=k.aPageStart,d.adStartTime=k[d.slotName]&&k[d.slotName].adStartTime||0,E(d.arid,d.slot,"defaultResizeSafeFrameHandler",I(d)),e=d.arid,t=d.slot,x[e]={slotId:t,placementDivId:"ape_"+t+"_placement",iframeId:"ape_"+t+"_iframe"},"clickTracking"in d&&""===d.clickTracking&&(d.clickTracking=D.getSlotPlaceholder(d)),d.minWidthToPunt&&d.programIdsToCollapse&&-1<d.programIdsToCollapse.indexOf(d.adCreativeMetaData.adProgramId)&&l.offsetWidth<d.minWidthToPunt){_(d.slot,null,"puntOnMinWidth:beforeRendering",1),B(x[d.arid].placementDivId);continue}if(d.forcePunt){P("forcePunt",d.slot,d.placementId),B(x[d.arid].placementDivId);continue}if(d.safeFrameSrc="true"!==d.abpAcceptable||"1"!==d.abpStatus&&"-1"!==d.abpStatus?h:w,d.abpStatus)for(var f in P("ABPStatus"+j[d.abpStatus],d.slot),j)_(d.slot,d.placementId,S[f],d.abpStatus===f?1:0);d.collectSafeframeRTD=k.collectSafeframeRTD,F("af",d.slot,d.placementId),_(d.slot,d.placementId,T.START,1);var m={};if(m.hostDomain=d.hostDomain,m.allowedSizes=d.allowedSizes,m.allowedDomains=d.allowedDomains,m.queryParams=d.queryParams,m.aPageStart=d.aPageStart,m.adStartTime=d.adStartTime,m.safeFrameSrc=d.safeFrameSrc,m.abpStatus=d.abpStatus,"function"!=typeof k.postMessage){z(y,1),B(x[d.arid].placementDivId),m.postMessage="postMessageNotSupported";continue}z(b,1),H(d.loadAfter,M(d),0,n),m.postMessage="postMessageSupported",m.loadAfter=d.loadAfter,A[d.arid].logTrace("additionalInitilizationParams",m)}catch(e){d=null,O("Error parsing sf tag",e)}}},k.DAsf.loadAds()}function E(e,t,a,n){o[e]=o[e]||{},o[e][a]=s.safeFunctionWrapper(n,O,"Error within ad handler "+a+": "+t)}function C(){var e={};try{for(var t=k.location.search.substring(1).split("&"),a=0;a<t.length;a++){var n=t[a].split("="),o=n[0];1<n.length&&0===o.indexOf("sf-")&&(e[o]=n[1])}}catch(e){O("Error parsing query parameters",e)}return e}function I(e){return function(){u(e.arid,e.size.width,e.size.height,e.maxAdWidth,e.adCreativeMetaData.adProgramId,e.programIdsToCollapse,e.minWidthToPunt,N,x)}}function M(t){return s.safeFunctionWrapper(function(){var e={callbackOccurred:!0};e.loadAfter=t.loadAfter,A[t.arid].logTrace("pageCallBack",e),_(t.slot,t.placementId,T.CALLBACK,1),function(e,t){if(!e)return!1;var a=L.getElementById(e);if(a&&!a.innerHTML){var n=a.getAttribute(v);if(n&&n===t.arid)return!0}return!1}(x[t.arid].placementDivId,t)&&function(t){try{var e=L.getElementById(x[t.arid].placementDivId),a={},n=x[t.arid].iframeId,o=t.safeFrameSrc,i=l(t,n,o);i.onload=function(){r.checkCache(t.DAsfUrl,t.safeFrameSrc,t.slot,t.placementId,_)},e.appendChild(i),F("cf",t.slot,t.placementId),_(t.slot,t.placementId,T.IFRAME_CREATED,1),N.adMap[t.arid]={slot:e,iframe:i,options:t},a.id=i.id,a.src=i.src,a.scrolling=i.scrolling,a.height=i.height,a.width=i.width,a.className=i.className,a.styleCssText=i.style.cssText,a.sandbox=i.sandbox,A[t.arid].logTrace("createSafeFrame",a)}catch(e){O("Error creating safeFrame",e),A[t.arid]&&A[t.arid].logTrace("createSafeFrame",{error:{message:"errorCreatingSafeFrame",ex:e}})}}(t)},O,"Error in callback to create Safeframe.")}}s.safeFunctionWrapper(function(){"undefined"==typeof JSON?o("https://images-na.ssl-images-amazon.com/images/G/01/da/js/json3.min._V308851628_.js",i):i()},O,"Error initializing safeFrame")()}(window,document)},{"../components/adBarTracer":5,"../components/cacheChecker":7,"../components/clickTrackingHelper":8,"./components/adFeedback":11,"./messenger/msgHandler":13,"./metrics/counters":14,"./metrics/csm":15}],11:[function(e,t,a){var w=e("../metrics/csm").sendCsmCounter,v=e("../metrics/counters"),y=e("../../components/ajaxRequest"),b=e("../../components/util").setObjectStyles;t.exports.appendAdFeedbackLinkToIframe=function(e,o,i){var t,a={};if(a.isFeedbackLoaded=e.isFeedbackLoaded,e&&!e.isFeedbackLoaded&&o.adFeedbackInfo.boolFeedback){e.isFeedbackLoaded=!0;var n=e.parentNode,r=o.placementId,s=o.adFeedbackInfo.slugText,d=o.adFeedbackInfo.endPoint,c=o.advertisementStyle,l=o.feedbackDivStyle,u=v.FEEDBACK_COUNTERS,p={adPlacementMetaData:o.adPlacementMetaData,adCreativeMetaData:o.adCreativeMetaData};a.slot=n,a.placementId=r,a.slugText=s,a.endPoint=d,a.advertisementStyle=c,a.feedbackDivStyle=l,a.adFeedbackParams=p;var f=function(e,t,a,n){var o=document.createElement(e);for(var i in t)o.setAttribute(i,t[i]);return b(o,a),n&&n.insertBefore(o,null),o},m=n.getElementsByTagName("div")[0]||f("div",{id:n.id+"_Feedback"},l,n),g=function(){i[o.arid].logTrace("adFeedBack",{renderFallbackAdvertisement:!0}),w(o.slot,r,u.FALLBACK,1),(m.getElementsByTagName("div")[0]||f("div",0,c,m)).innerHTML=s},h=d&&d.length?window.location.protocol+"//"+window.location.hostname+d+"?pl="+(t=p,encodeURIComponent(JSON.stringify(t))):d;a.requestUrl=h,i[o.arid].logTrace("adFeedBack",{adFeedbackRequest:a}),h?(w(o.slot,r,u.REQUEST,1),y.sendAjaxRequest(h,"GET",null,null,function(e){var t={feedbackResponseStarted:!0};if(4===e.readyState){if(200===e.status)try{var a=e.responseText,n=JSON.parse(a);(t.response=n)&&"ok"===n.status?("html"in n&&n.html&&(m.innerHTML=n.html),"script"in n&&n.script&&((m.getElementsByTagName("script")[0]||f("script",0,null,m)).innerHTML=n.script),w(o.slot,r,u.SUCCESS,1),t.feedBackResponseReturned=!0):g()}catch(e){g()}else t.feedBackResponseReturned=!1,g();i[o.arid].logTrace("adFeedBack",{adFeedBackResponse:t})}},g)):g()}}},{"../../components/ajaxRequest":6,"../../components/util":9,"../metrics/counters":14,"../metrics/csm":15}],12:[function(e,d,t){function c(e,t,a){var n=0;return document.hidden?n:(n=0<e?a-e:0<t?Math.min(t,a):0,Math.min(n,t-e))}function r(){try{var e={};return e.t=window.screenY?window.screenY:window.screenTop,e.l=window.screenX?window.screenX:window.screenLeft,e.w=d.exports.windowWidth(),e.h=d.exports.windowHeight(),e.b=e.t+e.h,e.r=e.l+e.w,e}catch(e){return null}}function s(e,t){try{var a={},n=function(e,t){try{var a={},n=t||e.getBoundingClientRect();return a.t=n.top,a.l=n.left,a.r=n.right,a.b=n.bottom,a.w=n.width||a.r-a.l,a.h=n.height||a.b-a.t,a.z=e?Number(window.getComputedStyle(e,null).zIndex):NaN,a}catch(e){return null}}(e,t),o=function(e){try{var t={},a=d.exports.windowWidth(),n=d.exports.windowHeight(),o=Math.max(0,c(e.t,e.b,n)),i=Math.max(0,c(e.l,e.r,a)),r=o*i,s=e.h*Math.min(e.w,d.exports.windowWidth());return t.xiv=Number(Math.min(1,i/e.w).toFixed(2)),t.yiv=Number(Math.min(1,o/e.h).toFixed(2)),t.iv=Number(Math.min(1,Math.max(0,r/s)).toFixed(2)),t}catch(e){return null}}(n);return a.t=n.t,a.l=n.l,a.r=n.r,a.b=n.b,a.w=n.w,a.h=n.h,a.z=n.z,a.xiv=o.xiv,a.yiv=o.yiv,a.iv=o.iv,a}catch(e){return null}}function l(e,t){try{var a={},n=t||e.getBoundingClientRect();return a.t=n.top,a.l=n.left,a.r=d.exports.windowWidth()-n.right,a.b=d.exports.windowHeight()-n.bottom,a.xs=Math.max(document.body.scrollWidth,document.documentElement.scrollWidth)>d.exports.windowWidth()?1:0,a.yx=Math.max(document.body.scrollHeight,document.documentElement.scrollHeight)>d.exports.windowHeight()?1:0,a}catch(e){return null}}d.exports.findVerticalPositionReached=function(){try{return window.scrollY+d.exports.windowHeight()}catch(e){return null}},d.exports.findDistanceFromViewport=function(e){try{return e.getBoundingClientRect().top-d.exports.windowHeight()}catch(e){return null}},d.exports.getViewableInfo=function(e){if(!e)return null;var t={},a=r(),n=s(e),o=l(e);return a&&n&&o?(t.geom={},t.geom.win=a,t.geom.self=n,t.geom.exp=o,t.payload={},t.payload.wh=a.h,t.payload.ww=a.w,t.payload.sx=window.scrollX,t.payload.sy=window.scrollY,t.payload.ah=n.h,t.payload.aw=n.w,t.payload.top=n.t,t.payload.left=n.l,t):null},d.exports.takeSnapshotOfSlotPosition=function(e){try{return{initialBoundingRect:e.getBoundingClientRect(),adHeight:e.offsetHeight,adWidth:e.offsetWidth,originalScrollX:window.scrollX,originalScrollY:window.scrollY}}catch(e){return null}},d.exports.getNoInventoryViewabilityData=function(e){var t={},a=function(e){try{var t=e.initialBoundingRect,a=t.top-(window.scrollY-e.originalScrollY),n=a+e.adHeight,o=t.left-(window.scrollX-e.originalScrollX),i=o+e.adWidth;return{top:a,bottom:n,left:o,right:i,width:e.adWidth,height:e.adHeight}}catch(e){return null}}(e),n=r(),o=s(null,a),i=l(null,a);return n&&o&&i?(t.geom={},t.geom.win=n,t.geom.self=o,t.geom.exp=i,t.payload={},t.payload.wh=n.h,t.payload.ww=n.w,t.payload.sx=window.scrollX,t.payload.sy=window.scrollY,t.payload.ah=o.h,t.payload.aw=o.w,t.payload.top=o.t,t.payload.left=o.l,t):null},d.exports.windowHeight=function(){return window.innerHeight||document.documentElement.clientHeight},d.exports.windowWidth=function(){return window.innerWidth||document.documentElement.clientWidth}},{}],13:[function(e,t,a){var w=e("../components/viewability"),p=e("../../components/util"),n=e("../metrics/csm"),o=n.sendCsmLatencyMetric,v=n.sendCsmCounter,i={ERROR:"ERROR",WARN:"WARN",FATAL:"FATAL"},l=r();function y(e,t){var a=t||new Error(e);v("",null,"safeFrameError",1),window.sfLogErrors&&(window.ueLogError?window.ueLogError(a,{logLevel:i.ERROR,attribution:"APE-safeframe",message:e+" "}):"undefined"!=typeof console&&console.error&&console.error(e,a))}function r(){var e=window.location.host.match(/^.*\.([^.:/]*)/),t=null;if(e&&1<e.length&&(t=e[1]),!/s/.test(location.protocol))return"cn"===t?"http://g-ec4.images-amazon.com":"http://z-ecx.images-amazon.com";var a="na";return/^(com|ca|mx)$/.test(t)?a="na":/^(uk|de|fr|it|es|in|ae|sa)$/.test(t)?a="eu":/^(jp|au)$/.test(t)?a="fe":/^(cn)$/.test(t)&&(a="cn"),"https://www.stormlin.com"}function u(e){return e.replace(/^.{1,5}:\/\/|^\/\//,"")}function b(e){var t=document.getElementById(e);void 0!==t&&t&&(t.style.display="none")}function f(e,t,a,n){var o=!1,i=function(){n(a,e)&&(t(),o=!0)},r=p.safeFunctionWrapper(p.throttle(function(){i(),o&&(p.removeWindowListener("scroll",i),p.removeWindowListener("resize",i))},20));p.addWindowListener("scroll",r),p.addWindowListener("resize",r)}t.exports.util=p,t.exports.viewability=w,t.exports.messenger=new function(e,t,a){var c=this;this.adMap=e||{},this.supportedCommands=t||{},this.msgListeners=a||{};var r=function(e){var t=c.adMap,a=t[e].options;if(t==={}||a==={})return null;var n="ape_"+a.slot+"_iframe";return t[e].iframe&&(t[e].iframe=t[e].iframe&&t[e].iframe.innerHTML?t[e].iframe:document.getElementById(n)),t[e].iframe};this.sendMessageToAd=function(e,t,a){var n=r(e),o=n?n.contentWindow:null;if(o){var i={command:t,data:a};i=JSON.stringify(i),o.postMessage(i,"*")}},this.receiveMessage=function(t){var e=c.adMap,a=c.supportedCommands;if(e!=={}){var n,o,i,r,s;try{if(t.data&&t.data.message&&/.*Mash.*/i.test(t.data.message.id))throw"Received Mash message";o=e[(n=JSON.parse(t.data)).arid]}catch(e){return}try{if(s=t,!(r=o)||!r.options||u(s.origin)!==u(l)||"object"!=typeof n.data)throw"Invalid Message: "+JSON.stringify(t.data);var d=a[n.command];d&&(o.options.debug&&"undefined"!=typeof console&&console.log(t),d(o,n.data))}catch(e){i="Problem with message: "+t.data,void 0!==n&&(i+=c.appendErrorDetails(n.arid)),y(i,e)}}},this.appendErrorDetails=function(e){var t=c.adMap;if(t==={})return"";var a="";if(void 0!==t[e]){var n=t[e].options;void 0!==n.aanResponse&&(a=" Ad Details: "+JSON.stringify(n.aanResponse))}return a},this.customMessage=function(e,t){var a=c.msgListeners;if(a[e])try{a[e](t)}catch(e){y("Custom Message Error",e)}else y("Unrecognized custom message key: "+e)},this.registerCustomMessageListener=function(e,t,a){var n=!1,o=c.msgListeners;try{!o[e]||"function"!=typeof o[e]||a?(o[e]=t,n=!0):y("Duplicate Key",new Error("Custom message listener already exists for key: "+e))}catch(e){y("Error registering custom message listener",e)}return n},this.sendCustomMessage=function(e,t){var a=c.adMap,n={key:e,data:t};for(var o in a)c.sendMessageToAd(o,"customMessage",n)},this.sendCustomMessageToAd=function(e,t,a){var n={key:t,data:a};c.sendMessageToAd(e,"customMessage",n)},this.takeSnapshotOfSlotPosition=function(e){var t=c.adMap,a=t&&t[e]&&t[e].options;if(t&&t!=={}&&a&&a!=={}){var n=r(e);c.adMap[e].options.slotSnapshot=w.takeSnapshotOfSlotPosition(n)}},this.updateViewability=function(e){var t=c.adMap,a=t&&t[e]&&t[e].options;if(t&&t!=={}&&a&&a!=={}){var n=r(e),o=t[e].options.viewabilityStandards,i=w.getViewableInfo(n);null!==i&&(i.viewabilityStandards=o,c.sendMessageToAd(e,"updateViewability",i))}},this.updateNoInventoryViewability=function(e){var t=c.adMap,a=t&&t[e]&&t[e].options,n=a&&a.slotSnapshot;if(t&&t!=={}&&a&&a!=={}&&n){var o=a.viewabilityStandards,i=w.getNoInventoryViewabilityData(n);null!==i&&(i.viewabilityStandards=o,c.sendMessageToAd(e,"updateViewability",i))}}},t.exports.logError=y,t.exports.SF_DOMAIN=l,t.exports.loadScript=function(e,t){var a=document.createElement("script");a.src=e,a.setAttribute("crossorigin","anonymous"),a.onload=a.onreadystatechange=function(){a.readyState&&"loaded"!==a.readyState&&"complete"!==a.readyState||(a.onload=a.onreadystatechange=null,t&&"function"==typeof t&&t())},a.onerror=function(e){return y("Error loading script",e),!1},(document.getElementsByTagName("head")[0]||document.getElementsByTagName("body")[0]).appendChild(a)},t.exports.fireViewableLatencyMetrics=function(e,t,a,n){window.apeViewableLatencyTrackers&&window.apeViewableLatencyTrackers[e]&&window.apeViewableLatencyTrackers[e].valid&&(window.apeViewableLatencyTrackers[e].loaded=!0,window.apeViewableLatencyTrackers[e].viewed&&(o("ld",t,a,"viewablelatency",n),v(t,a,"htmlviewed:loaded",1)))},t.exports.hasClass=function(e,t){var a=new RegExp("(^|\\s)"+t+"(\\s|$)"),n=e.className;return n&&a.test(n)},t.exports.createIframeWithAttributes=function(e,t,a){var n,o=JSON.stringify(e);if(/MSIE (6|7|8)/.test(navigator.userAgent))try{n=document.createElement("<iframe name='"+o+"'>")}catch(e){(n=document.createElement("iframe")).name=o}else(n=document.createElement("iframe")).name=o;return n.id=t,n.src=a,n.height=e.size.height||"250px",n.width=e.size.width||"300px",n.className=e.iframeClass||"",n.setAttribute("frameborder","0"),n.setAttribute("marginheight","0"),n.setAttribute("marginwidth","0"),n.setAttribute("scrolling","no"),n.setAttribute("allowtransparency","true"),n.setAttribute("allowfullscreen",""),n.setAttribute("mozallowfullscreen",""),n.setAttribute("webkitallowfullscreen",""),n.setAttribute("data-arid",e.arid),n.style.cssText=e.iframeExtraStyle||"",n.sandbox="allow-scripts allow-top-navigation allow-popups allow-popups-to-escape-sandbox allow-same-origin",n},t.exports.logCounter=function(e,t){window.ue&&"function"==typeof window.ue.count&&window.ue.count(e,t)},t.exports.collapseSlot=b,t.exports.resizeSafeFrameAd=function(t,a,n,e,o,i,r,s,d){try{var c=document.getElementById(d[t].placementDivId),l=document.getElementById(d[t].wrapperDivId)||c,u=document.getElementById(d[t].iframeId);if(null===l||null===c||null===u)return;if(r&&i&&-1<i.indexOf(o)&&0<c.offsetWidth&&c.offsetWidth<r)return v(d[t].slotId,null,"puntOnMinWidth:afterRendering",1),void b(d[t].placementDivId);var p=n,f=a,m=function(e){p=Math.round(e*n/a),f=Math.round(e)},g=0===l.offsetWidth?w.windowWidth():l.offsetWidth;e&&w.windowHeight()<w.windowWidth()?m(e):m(g),s&&s.adMap&&s.adMap[t]&&s.adMap[t].options&&s.adMap[t].options.slotSnapshot&&(s.adMap[t].options.slotSnapshot.adHeight=p,s.adMap[t].options.slotSnapshot.adWidth=f),p+="px",f+="px",u.style.height=p;var h={width:u.style.width=f,height:p};l!==c&&(c.style.height=p,s.sendMessageToAd(t,"resizeCreativeWrapper",h)),"Detail_hero-quick-promo_Desktop"===d[t].slotId&&(v(d[t].slotId,s.adMap[t].options.placementId,"OffsetLeft",u.offsetLeft),v(d[t].slotId,s.adMap[t].options.placementId,"OffsetTop",u.offsetTop),v(d[t].slotId,s.adMap[t].options.placementId,"OffsetWidth",u.offsetWidth),v(d[t].slotId,s.adMap[t].options.placementId,"OffsetHeight",u.offsetHeight))}catch(e){y("Error resizing ad: "+d[t].slotId,e)}},t.exports.delayLoad=function(e,t,a,n){var o="undefined"!=typeof P,i="undefined"!=typeof amznJQ,r="number"==typeof a&&0!==a?function(){setTimeout(t,a)}:t;if("windowOnLoad"===e)"complete"===document.readyState?r():p.addWindowListener("load",r);else if("spATFEvent"===e)o?P.when("search-page-utilities").execute(function(e){e.afterEvent("spATFEvent",r)}):i?amznJQ.available("search-js-general",function(){window.SPUtils.afterEvent("spATFEvent",r)}):r();else if("aboveTheFold"===e)o?P.when("af").execute(r):i?amznJQ.onCompletion("amznJQ.AboveTheFold",r):r();else if("criticalFeature"===e)o?P.when("cf").execute(r):i?amznJQ.onCompletion("amznJQ.criticalFeature",r):r();else if("r2OnLoad"===e)o?P.when("r2Loaded").execute(r):i?amznJQ.onReady("r2Loaded",r):r();else if(e.match("[^:]+:.+")){var s=e.split(":"),d=s[0].split("."),c=s[1],l=2<s.length?s[2]:c;o?P.when(l,"A").execute(r):i&&1<d.length?amznJQ[d[1]](c,r):r()}else if(e.match(/^\d{1,4}px$/g))f(parseInt(e,10),r,n,function(e,t){return e&&w.findDistanceFromViewport(e)<=t});else{var u=/^reached(\d{1,5}px)FromTop$/g.exec(e);u?f(parseInt(u[1],10),r,n,function(e,t){return w.findVerticalPositionReached()>=t}):r()}},t.exports.getMediaCentralOrigin=r,t.exports.appendJsScript=function(e,t,a){var n=document.createElement("script");n.charset=a||"utf-8",n.src=e,t.appendChild(n)},t.exports.scriptValidator=function(e,t){return e.match(/^((?:https?:)?\/\/)?([\w\-\.]+(?::[0-9]+)?)\/?(.*)$/)[2]===t},t.exports.sizeValidator=function(e,t){return e.height===t.height&&e.width===t.width},t.exports.checkAgainstWhitelist=function(e,t,a){if(!t||"object"!=typeof t)return!1;for(var n=0,o=t.length;n<o;n++)if(a(e,t[n]))return!0;return!1}},{"../../components/util":9,"../components/viewability":12,"../metrics/csm":15}],14:[function(e,t,a){t.exports.AD_LOAD_COUNTERS={START:"adload:start",CALLBACK:"adload:delayloadcallback",IFRAME_CREATED:"adload:iframecreated"},t.exports.CACHE_COUNTERS={SF_LIBRARY:"cache:sflibrary:",SF_HTML:"cache:sfhtml:"},t.exports.FEEDBACK_COUNTERS={REQUEST:"adfeedback:request",SUCCESS:"adfeedback:success",FALLBACK:"adfeedback:fallback"},t.exports.MESSENGER_COUNTERS={API:"messenger:"},t.exports.ABP_STATUS_COUNTERS={1:"abpstatus:enabled",0:"abpstatus:notenabled","-1":"abpstatus:unknown"},t.exports.SF_VERSION_COUNTERS={VERSION:"sfversion"},t.exports.RESOURCE_TIMING_DATA_COUNTERS={NEXUS_CLIENT_NOT_DEFINED:"ResourceTimingData.NexusClientNotDefined",LOGGING_FAILED:"ResourceTimingData.LoggingFailed",LOGGING_SUCCESSFUL:"ResourceTimingData.LoggingSuccessful",DEPENDENCIES_NOT_MET:"ResourceTimingData.DependenciesNotMet"}},{}],15:[function(e,t,a){var s={bb:"uet",af:"uet",cf:"uet",be:"uet",ld:"uex"};function d(e,t,a,n){var o=[e,t,a];return n&&o.push(n),o}t.exports.sendCsmLatencyMetric=function(e,t,a,n,o){var i=s[e],r=n?n+":":"";"function"==typeof window[i]&&(window[i].apply(this,d(e,"adplacements:"+r+t.replace(/_/g,":"),{wb:1},o)),a&&window[i].apply(this,d(e,"adplacements:"+r+a,{wb:1},o)))},t.exports.sendCsmCounter=function(e,t,a,n){if(window.ue&&"function"==typeof window.ue.count){var o="adplacements:"+a;if(e&&(o+=":"+e.replace(/_/g,":")),window.ue.count(o,n),t){var i="adplacements:"+(a&&t?a+":":a)+t;window.ue.count(i,n)}}},t.exports.addCsmTag=function(e,t,a,n){if(window.ue&&window.ue.tag){var o=e+":"+t.replace(/_/g,":")+(n?":"+n:"");if(window.ue.tag(o),a){var i=e+":"+a+(n?":"+n:"");window.ue.tag(i)}}}},{}]},{},[10]);