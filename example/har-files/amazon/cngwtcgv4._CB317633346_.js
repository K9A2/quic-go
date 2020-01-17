function cnGwtcgjs1(navShowTop, height, jsParams, urlGetContent){
if(!window.ccSlideShowV2) {
window.ccSlideShowV2 = {
  sliderShowInterval : -1,
  slideShowtimeOut : 4000,
  fadeOutTime : 900,
  fadeInTime : 900,
  
  navShowTime: 200,
  navHideTime: 200,
  navShowTop : navShowTop,
  navHiddenTop : height,
  
  defaultHref : "http://www.amazon.cn/",
  
  enableImgOnErr : 1,
  maxImgPreLoadErrorTryCount : 2,
  enableImgHover : 1,
  widthRatio: 100,
  
  totalItemSize : 0,
  curSliderOrderInCurFrame : 0,
  curFrameIndex : 0,
  lastSliderOrderInCurFrame : 0,
  
  isWaitForFirstImg : 0,
  isOpenNewTab : 1,
  useAjax : 1
};
ccSlideShowV2.beginTime=+new Date();
ccSlideShowV2.setIntervalTrigger = function() {
  ccSlideShowV2.isNeedPause = false;
}
ccSlideShowV2.clearIntervalTrigger = function() {
  ccSlideShowV2.isNeedPause = true;  
  if(ccSlideShowV2.pauseStopCount==0 && ccSlideShowV2.needAddPauseTime) ccSlideShowV2.pauseStopCount=1;
}
ccSlideShowV2.updateSlideShowInformation = function(){
  var $=ccSlideShowV2.jQuery;
  ccSlideShowV2.totalItemSize = $(".cc-lm-tcgImgItem").length;
  
  ccSlideShowV2.imgPreLoadState = new Array( ccSlideShowV2.totalItemSize);
  ccSlideShowV2.imgPreLoadState[0] = 1;
  ccSlideShowV2.imgPreLoadErrorTryCount = new Array( ccSlideShowV2.totalItemSize);
  for(var i=0; i<ccSlideShowV2.totalItemSize; i++) { ccSlideShowV2.imgPreLoadErrorTryCount[i] = 0; }
  ccSlideShowV2.imgSrcState = new Array( ccSlideShowV2.totalItemSize);
  ccSlideShowV2.imgSrcState[0] = 1;
  
  if(ccSlideShowV2.totalFrameCount<2) {
    $("#cc-lm-prevItem").css("display", "none");
    $("#cc-lm-nextItem").css("display", "none");
    $("#cc-lm-tcgShowIndicatorContainer").css("display", "none");    
  } else {
    ccSlideShowV2.lastSliderOrderInCurFrame = ccSlideShowV2.totalItemSize - 1;
    ccSlideShowV2.widthRatio = 100/ccSlideShowV2.totalItemSize;
    $("#cc-lm-tcgShowIndicatorContainer").css("display", "block");
    $("#cc-lm-tcgShowIndicator").css("width", ccSlideShowV2.widthRatio+"%");        
    $("#cc-lm-WordSepLine li").css("width", ccSlideShowV2.widthRatio+"%");
    $("#cc-lm-navItems li").css("width", ccSlideShowV2.widthRatio+"%");
  }
}
ccSlideShowV2.preLoadImage = function(index){
  if(ccSlideShowV2.imgPreLoadState[index]) return;  
  var $=ccSlideShowV2.jQuery;  
  var imgItem = $(".cc-lm-tcgImgItem img:eq("+index+")"); 
  var imgSrc = imgItem.attr("srcdata-300") || imgItem.prop("srcdata-300");
  if(typeof imgSrc == 'undefined') 
    imgSrc = imgItem.attr("srcdata") || imgItem.prop("srcdata");
  if(typeof imgSrc == 'undefined') return;
  var img = new Image();
  img.src = imgSrc;
  img.onload = function() {
    ccSlideShowV2.imgSrcState[index] = 1;
    imgItem.attr("src", imgSrc);    
  };
  ccSlideShowV2.imgPreLoadState[index] = 1;
}
ccSlideShowV2.reloadImg = function(index){
  var $=ccSlideShowV2.jQuery;  
  var imgItem = $(".cc-lm-tcgImgItem img:eq("+index+")");
  imgItem.attr("onerror", ""); 
  if(ccSlideShowV2.imgPreLoadErrorTryCount[index]>ccSlideShowV2.maxImgPreLoadErrorTryCount)     
    return;  
  ccSlideShowV2.imgPreLoadErrorTryCount[index]++;
  
  var imgSrc = imgItem.attr("srcdata-300") || imgItem.prop("srcdata-300");
  if(typeof imgSrc == 'undefined') 
    imgSrc = imgItem.attr("srcdata") || imgItem.prop("srcdata");
  if(typeof imgSrc == 'undefined') return;
  imgItem.attr("src", ""); 
  ccSlideShowV2.imgSrcState[index] = 0;
  ccSlideShowV2.preLoadImage(index);
}
ccSlideShowV2.checkAndSetImgSrc = function(index){
  if(ccSlideShowV2.imgSrcState[index]) return;  
  var $=ccSlideShowV2.jQuery;  
  var imgItem = $(".cc-lm-tcgImgItem img:eq("+index+")"); 
  var imgSrc = imgItem.attr("srcdata") || imgItem.prop("srcdata");
  if(typeof imgSrc == 'undefined') return;
  imgSrc = imgSrc;
  imgItem.attr("src", imgSrc);
  if(ccSlideShowV2.enableImgOnErr)
    imgItem.attr("onerror", "ccSlideShowV2.reloadImg("+index+");");
  ccSlideShowV2.imgSrcState[index] = 1;
}
ccSlideShowV2.initCallBackUrls = function(callbackUrls) {
  //TODO ignore the useAjax param
  ccSlideShowV2.cbUrls = new Array(callbackUrls.length+1);
  ccSlideShowV2.cbUrlsUpdated = new Array(callbackUrls.length+1);
  ccSlideShowV2.cbUrls[0] = '';
  ccSlideShowV2.cbUrlsUpdated[0] = 0;
  for(var i=1; i<ccSlideShowV2.cbUrls.length; i++) {
    ccSlideShowV2.cbUrls[i] = callbackUrls[i-1];
    ccSlideShowV2.cbUrlsUpdated[i] = 0;
  }
}
ccSlideShowV2.updateCallback = function(order) {
  if(order<0 || order>=ccSlideShowV2.cbUrls.length) return;
  if(ccSlideShowV2.cbUrlsUpdated[order]) return;
  
  ccSlideShowV2.cbUrlsUpdated[order] = 1;
  if(ccSlideShowV2.cbUrls[order].length>0) ccSlideShowV2.jQuery.get(ccSlideShowV2.cbUrls[order]);
}
ccSlideShowV2.changeSlide = function(curFrameIndex, curSliderOrderInCurFrame){    
  var $=ccSlideShowV2.jQuery;    
  var newSliderIndex = curSliderOrderInCurFrame;
  ($(".cc-lm-tcgImgItem:not(:eq("+newSliderIndex+")):visible")).fadeOut(ccSlideShowV2.fadeOutTime);
  ($("#cc-lm-navItems li").not(":eq("+newSliderIndex+")")).removeClass("on");
  ($("#cc-lm-navItems li:eq("+newSliderIndex+")")).addClass("on");
  ($("#cc-lm-tcgShowIndicator")).css("left", ccSlideShowV2.widthRatio*curSliderOrderInCurFrame+"%");
  ccSlideShowV2.checkAndSetImgSrc(newSliderIndex);
  ($(".cc-lm-tcgImgItem:eq("+newSliderIndex+")")).fadeIn(ccSlideShowV2.fadeInTime);
  var nextNewSliderIndex = newSliderIndex+1;
  ccSlideShowV2.preLoadImage(nextNewSliderIndex);
  ccSlideShowV2.updateCallback(newSliderIndex);
}
ccSlideShowV2.sliderShow = function(){
  if(ccSlideShowV2.isNeedPause) {
    return;
  }
  if(ccSlideShowV2.pauseStopCount>0) {
    ccSlideShowV2.pauseStopCount = -1;
    return;
  }
  ccSlideShowV2.pauseStopCount = 0;
  var $=ccSlideShowV2.jQuery;
  
  if(ccSlideShowV2.curSliderOrderInCurFrame==ccSlideShowV2.lastSliderOrderInCurFrame) {
    ccSlideShowV2.curSliderOrderInCurFrame=0;
  } else {
    ccSlideShowV2.curSliderOrderInCurFrame++;
  }     
  ccSlideShowV2.changeSlide(ccSlideShowV2.curFrameIndex, ccSlideShowV2.curSliderOrderInCurFrame);
}
ccSlideShowV2.moveToHoverdItem = function(hoverItem) {
  var $=ccSlideShowV2.jQuery;
  ccSlideShowV2.endAllAnimate();
      
  var hoveredListOrder = hoverItem.attr("order") || hoverItem.prop("order") ;
  if(typeof hoveredListOrder == 'undefined') return;
  
  var hoveredSlideShowIndex = parseInt(hoveredListOrder);
  var curSliderIndex = ccSlideShowV2.curSliderOrderInCurFrame;
  if(curSliderIndex == hoveredSlideShowIndex) return;
  
  ccSlideShowV2.curSliderOrderInCurFrame = hoveredListOrder;
  ccSlideShowV2.changeSlide(ccSlideShowV2.curFrameIndex, ccSlideShowV2.curSliderOrderInCurFrame);
}
ccSlideShowV2.hideWordSepLine = function() {
  var $=ccSlideShowV2.jQuery;
  $("#cc-lm-WordSepLine").css("display", "none");
}
ccSlideShowV2.updateWordSepLine = function(frameIndex) {
  var $=ccSlideShowV2.jQuery;
  $("#cc-lm-WordSepLine li:gt("+ccSlideShowV2.lastSliderOrderInCurFrame+")").css("display", "none");
}
ccSlideShowV2.endAllAnimate = function() {
  var $=ccSlideShowV2.jQuery;  
  ccSlideShowV2.clearIntervalTrigger();
  ($("#cc-lm-navItemsContainer")).stop(true, true);
  $(".cc-lm-tcgImgItem").stop(true, true);
  ccSlideShowV2.clearIntervalTrigger();
}
ccSlideShowV2.showNavItems = function() {
  var $=ccSlideShowV2.jQuery;
  $("#cc-lm-navItemsContainer").animate(
    {top:ccSlideShowV2.navShowTop+"px"},
    ccSlideShowV2.navShowTime,
    function(){      
    });
}
ccSlideShowV2.hideNavItems = function() {
  var $=ccSlideShowV2.jQuery;
  $("#cc-lm-navItemsContainer").animate(
    {top:ccSlideShowV2.navHiddenTop+"px"},
    ccSlideShowV2.navHideTime,
    function(){      
    });
}
ccSlideShowV2.slideTouchEventHandle = function(e) {
  if(!ccSlideShowV2.isMoving)
    return;
  var x=(e.touches && e.touches.length == 1)?e.touches[0].pageX:e.pageX;
  var y=(e.touches && e.touches.length == 1)?e.touches[0].pageY:e.pageY;
  var dx=x-ccSlideShowV2.touchMoveStartX;
  var dy=y-ccSlideShowV2.touchMoveStartY;
  if(Math.abs(dx)<Math.abs(dy) || dx==0) {
    ($(this)).unbind("touchmove");($(this)).unbind("touchend");
    ccSlideShowV2.isMoving = false;
  } else {
    e.preventDefault();
  }
  if(Math.abs(dx)<16)
    return;
  var $=ccSlideShowV2.jQuery;
  ccSlideShowV2.endAllAnimate();
  if(dx>0) {//right
    if(ccSlideShowV2.curSliderOrderInCurFrame==ccSlideShowV2.lastSliderOrderInCurFrame) {
      ccSlideShowV2.curSliderOrderInCurFrame=0;
    } else {
      ccSlideShowV2.curSliderOrderInCurFrame++;
    } 
  } else {//left
    if(ccSlideShowV2.curSliderOrderInCurFrame==0) {
      ccSlideShowV2.curSliderOrderInCurFrame=ccSlideShowV2.lastSliderOrderInCurFrame;
    } else {
      ccSlideShowV2.curSliderOrderInCurFrame--;
    }
  }  
  ccSlideShowV2.changeSlide(ccSlideShowV2.curFrameIndex, ccSlideShowV2.curSliderOrderInCurFrame);
  ($(this)).unbind("touchmove");($(this)).unbind("touchend");
  ccSlideShowV2.setIntervalTrigger();
  ccSlideShowV2.isMoving = false;
}
ccSlideShowV2.updateEventInfo = function() {
  var $=ccSlideShowV2.jQuery; 
  
  var hasTouchEvent = "ontouchstart" in window || "ontouchstart" in document;
  var clickEvent = "click";
  if(hasTouchEvent) clickEvent = "touchstart";
  
  $("#cc-lm-tcgShowImgContainer").bind("touchstart",
    function(e) {      
      ccSlideShowV2.touchMoveStartX = (e.touches && e.touches.length == 1)?e.touches[0].pageX:e.pageX;
      ccSlideShowV2.touchMoveStartY = (e.touches && e.touches.length == 1)?e.touches[0].pageY:e.pageY;
      ccSlideShowV2.isMoving = true;
      ($(this)).bind('touchmove', ccSlideShowV2.slideTouchEventHandle);
      ($(this)).bind('touchend', function(e){($(this)).unbind("touchmove");($(this)).unbind("touchend");});            
    }
  );
    
  if(ccSlideShowV2.enableImgHover) {  
    $("#cc-lm-tcgShowContainer").hover(
      function() {
        ccSlideShowV2.endAllAnimate();
        ccSlideShowV2.showNavItems();
      },
      function() {
        ccSlideShowV2.hideNavItems();
        ccSlideShowV2.setIntervalTrigger();
      }
    );   
    $(".cc-lm-tcgImgItem").click(
      function() {
        ccSlideShowV2.setIntervalTrigger();
      }
    );
  } 
  
  $("#cc-lm-prevItem").click(
    function(e) {
      ccSlideShowV2.endAllAnimate();
      if(ccSlideShowV2.curSliderOrderInCurFrame==0) {
        ccSlideShowV2.curSliderOrderInCurFrame=ccSlideShowV2.lastSliderOrderInCurFrame;
      } else {
        ccSlideShowV2.curSliderOrderInCurFrame--;
      }
      ccSlideShowV2.changeSlide(ccSlideShowV2.curFrameIndex, ccSlideShowV2.curSliderOrderInCurFrame);
      if(hasTouchEvent) ccSlideShowV2.setIntervalTrigger();
      return false;
    }
  ); 
  $("#cc-lm-nextItem").click(
    function(e) {      
      ccSlideShowV2.endAllAnimate();
      if(ccSlideShowV2.curSliderOrderInCurFrame==ccSlideShowV2.lastSliderOrderInCurFrame) {
        ccSlideShowV2.curSliderOrderInCurFrame=0;
      } else {
        ccSlideShowV2.curSliderOrderInCurFrame++;
      }
      ccSlideShowV2.changeSlide(ccSlideShowV2.curFrameIndex, ccSlideShowV2.curSliderOrderInCurFrame);
      if(hasTouchEvent) ccSlideShowV2.setIntervalTrigger();
      return false;
    }
  );
  
  if(!hasTouchEvent) {
    $("#cc-lm-navItems li").hover(
      function() {
        ccSlideShowV2.endAllAnimate();
        ccSlideShowV2.moveToHoverdItem($(this));
      },
      function() {
      }
    );  
  }
  $("#cc-lm-navItems li").bind(clickEvent, 
    function(event) {
      ccSlideShowV2.endAllAnimate();
      ccSlideShowV2.moveToHoverdItem($(this));
      if(hasTouchEvent) ccSlideShowV2.setIntervalTrigger();
      event.handled = true; 
      return false;
    }
  );
}
ccSlideShowV2.waitForFirstImgLoadAndBeginInterval = function() {    
  var $=ccSlideShowV2.jQuery;
  var img = new Image();
  var imgSrc = $(".cc-lm-tcgImgItem:eq(0)").css("background-image");
  imgSrc = imgSrc.substring(4,imgSrc.length-1);
  img.src = imgSrc;
  if(img.complete) {
    ccSlideShowV2.sliderShowInterval = setInterval("ccSlideShowV2.sliderShow()",ccSlideShowV2.slideShowtimeOut);
  } else {
    img.onload = function() {
      ccSlideShowV2.sliderShowInterval = setInterval("ccSlideShowV2.sliderShow()",ccSlideShowV2.slideShowtimeOut);
    };   
  }
}
ccSlideShowV2.setSliderShowInterval = function(){
  ccSlideShowV2.updateSlideShowInformation();
  ccSlideShowV2.preLoadImage(1);
  ccSlideShowV2.updateWordSepLine(0);
  if(ccSlideShowV2.useAjax==0 && ccSlideShowV2.isWaitForFirstImg==1)
    ccSlideShowV2.waitForFirstImgLoadAndBeginInterval();  
  else
    ccSlideShowV2.sliderShowInterval = setInterval("ccSlideShowV2.sliderShow()",ccSlideShowV2.slideShowtimeOut);
  
  ccSlideShowV2.updateEventInfo();
}

if (typeof amznJQ !== 'undefined') {
  amznJQ.available('jQuery', function() {         
    ccSlideShowV2.jQuery = jQuery;    
    amznJQ.declareAvailable('ccSlideShowV2'); 
    if(ccSlideShowV2.useAjax==0) jQuery(document).ready(ccSlideShowV2.setSliderShowInterval);
  });
} else if (typeof P !== 'undefined'){
	P.when('A').execute(function(A){
		ccSlideShowV2.jQuery = A.$;  
    P.declare('ccSlideShowV2');
		if(ccSlideShowV2.useAjax==0) A.$(document).ready(ccSlideShowV2.setSliderShowInterval);
	})
}
}
}
function cnGwtcgjs2(navShowTop, height, jsParams, urlGetContent){
if(!window.ccSSDriverV2) {
    window.ccSSDriverV2 = {};
    ccSSDriverV2.updateSlideAfterCF = 1;
    ccSSDriverV2.jsParams = jsParams;
    ccSSDriverV2.urlGetContent = urlGetContent;
    ccSSDriverV2.enableMinUpdateTime = 1;
    ccSSDriverV2.minUpdateTime = 2600;                  
    ccSSDriverV2.handleErr = function() {
    }

    ccSSDriverV2.updateSlideCount = 1;
    ccSSDriverV2.recUET = function(sMarker) {
      if(typeof uet == 'function') { uet(sMarker, 'cngwtcgaj', {wb: 1}); }
    }
    ccSSDriverV2.recUEX = function(sMarker) {
      if(typeof uex == 'function') { uex(sMarker, 'cngwtcgaj', {wb: 1}); }
    }      
    ccSSDriverV2.insertCCSlides = function($, contents, backgroundColors, categoryNames, showAds, cUrls){
      if(--ccSSDriverV2.updateSlideCount < 0) return;
      if(ccSSDriverV2.updateSlideAfterCF) ccSSDriverV2.recUET('af');    
      var curSlideCount = $(".cc-lm-tcgImgItem").length;
      for(var i=0; i<contents.length; i++) {
          var display = "none;";
          if(i==0 && curSlideCount==0) display="block;";
          var content = "<div class=\"cc-lm-tcgImgItem\" style=\"display: "+display+backgroundColors[i]+"\">" + contents[i] +"</div>";
          $("#cc-lm-tcgShowImgContainer").append(content);
      }
      var endSlideCount = categoryNames.length + curSlideCount;
      var widthRatio = 100/endSlideCount;
      for(var i=curSlideCount; i<endSlideCount-1; i++) {
          var curRatio = i*widthRatio;
          var content = "<li style='display:block; margin:0; padding:0; height:50px;left:"+curRatio+"%;top:0;position:absolute;'></li>";        
          $("#cc-lm-WordSepLine").append(content);
      } 
      $("#cc-lm-WordSepLine li").css("width", widthRatio+"%");
      
      var regBR=/<br>/ig;
      var regQuote=/"/ig;
      var hoverTitle;    
      for(var i=curSlideCount; i<endSlideCount; i++) {
          hoverTitle = "["+ categoryNames[i-curSlideCount] + "]" + showAds[i-curSlideCount];
          hoverTitle = hoverTitle.replace(regBR," ");
          hoverTitle = hoverTitle.replace(regQuote,"");
          var curRatio = i*widthRatio;
          var content = "<li order='"+i+"' style='display:block; margin:0; padding:0; text-align:center;height:50px;left:"+curRatio+"%;top:0;position:absolute;'>" +
            "<div style='cursor:pointer; width:80%; margin:auto; display: table; height:50px;'>" +
            "  <span class='cc-lm-navCategory' style='display: table-cell; vertical-align: middle; height:50px;overflow:hidden;' " +
                     'title="'+hoverTitle+'">' +
            "    <span style='display:block;width:auto;height:16px;margin-bottom:4px;overflow:hidden;'>" + categoryNames[i-curSlideCount] + "</span>" +          
            "    <span style='display:block;width:auto;height:16px;margin-top:3px;overflow:hidden;'><b>" + showAds[i-curSlideCount] + "</b></span>" +
            "  </span>"+
            "</div>"+
            "</li>";
          $("#cc-lm-navItems").append(content);
      }
      $("#cc-lm-navItems li").css("width", widthRatio+"%");  
      $("#cc-lm-tcgShowIndicator").css("width", widthRatio+"%");    
      if(contents.length>0) {
          if (typeof amznJQ !== 'undefined') {
              amznJQ.available('ccSlideShowV2', function() {
                  ccSlideShowV2.initCallBackUrls(cUrls);
                  ccSlideShowV2.setSliderShowInterval();
              });
          } else if (typeof P !== 'undefined') {
              P.when('ccSlideShowV2').execute(function(a) {
                  ccSlideShowV2.initCallBackUrls(cUrls);
                  ccSlideShowV2.setSliderShowInterval();
              });
          }
      } else {
          ccSSDriverV2.handleErr();
      }
      if(ccSSDriverV2.updateSlideAfterCF) ccSSDriverV2.recUET('cf');
    }
    ccSSDriverV2.getCCSlideInfos = function($) {
      var params = ccSSDriverV2.jsParams;
      $.ajax({
          type: 'POST',
          url: ccSSDriverV2.urlGetContent,
          data: params,
          dataType: 'text',
          cache: false,
          contentType: 'application/x-www-form-urlencoded; charset=utf-8',
          timeout: 6000,
          success: function(data) {
              var jnData = eval('(' + data + ')');
              if (jnData.success) {
                  ccSSDriverV2.recUET('bb');
                  if(ccSSDriverV2.updateSlideAfterCF) {
                    if (typeof amznJQ !== 'undefined') {
                      amznJQ.available('cf', function() {
                        ccSSDriverV2.insertCCSlides($, jnData.contents, jnData.backgroundColors, jnData.categoryNames, jnData.showAds, jnData.cUrls);
                      });
                    }else if (typeof P !== 'undefined') {
                      P.when('cf').execute(function() {
                       ccSSDriverV2.insertCCSlides($, jnData.contents, jnData.backgroundColors, jnData.categoryNames, jnData.showAds, jnData.cUrls);
                      });
                    }
                    if(ccSSDriverV2.enableMinUpdateTime) {
                      setTimeout(function(){
                        ccSSDriverV2.insertCCSlides($, jnData.contents, jnData.backgroundColors, jnData.categoryNames, jnData.showAds, jnData.cUrls);
                      }, ccSSDriverV2.minUpdateTime);
                    }
                  } else {
                    ccSSDriverV2.insertCCSlides($, jnData.contents, jnData.backgroundColors, jnData.categoryNames, jnData.showAds, jnData.cUrls);
                  }
                  ccSSDriverV2.recUEX('ld');
              }
          },
          error: function() {
              ccSSDriverV2.handleErr();
          }        
      });
    }
    if (typeof amznJQ !== 'undefined') {
      amznJQ.available('jQuery', function() {
          ccSSDriverV2.getCCSlideInfos(jQuery);
      });
    } else if (typeof P !== 'undefined') {
      P.when('A').execute(function(a) {
          ccSSDriverV2.getCCSlideInfos(a.$);
      });
    }
  }
}