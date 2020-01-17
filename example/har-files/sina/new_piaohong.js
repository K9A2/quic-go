/**
 * Created by sina on 18/02/28.
 */
(function(){
    //飘红配置
     if (typeof specialBg !== 'undefined' && specialBg === true) {
        var CONFIG = {
            // 活动标记，英文字母、数字、下划线任意组合，如：2015两会=sina_news_2015lainghui
            activityName: 'sina_news_2019_guoqing',
            // 活动名称
            activityChineseName: '',
            // 飘红高度
            headHeight: 80,
            // 普通背景
            normalBG: '',

            // 飘红1000px背景
            activitySMBG: window.smallNewsPic,
            // 关闭按钮背景
            closeBtnBG: '',
            // 关闭按钮hover背景，默认为closeBtnBG
            closeBtnHoverBG: '',
            // 关闭按钮离下边正文的高度单位px
            closeBtnBottom: 35,
            // 关闭按钮左侧链接
            leftLink: '//news.sina.com.cn/z/chunyun2020/',
            // 是否显示左侧按钮关闭链接, 1：显示，0：隐藏
            isShowLeftLink: 0,
        };
        // =======飘红配置结束==========

        // 检查参数
        if (!CONFIG.activityName) {
            return;
        };
        if (!CONFIG.closeBtnHoverBG) {
            CONFIG.closeBtnHoverBG = CONFIG.closeBtnBG;
        };
        if (!CONFIG.leftLink) {
            CONFIG.leftLink = '#url';
        };
    var pthis = this;
//获取对象
    this.$ = function(id){if(document.getElementById){return eval('document.getElementById("'+id+'")')}else{return eval('document.all.'+id)}};

//获取cookie
    this.getAdCookie = function(N){
        var c=document.cookie.split("; ");
        for(var i=0;i<c.length;i++){var d=c[i].split("=");if(d[0]==N)return unescape(d[1]);}
        return "";
    };
    this.getElementsByClass = function(searchClass, node, tag) {
        var classElements = [];
        if (node == null) node = document;
        if (tag == null) tag = '*';
        var els = node.getElementsByTagName(tag);
        var elsLen = els.length;
        var pattern = new RegExp("(^|\\s)"+searchClass+"(\\s|$)");
        for (i = 0, j = 0; i < elsLen; i++) {
            if (pattern.test(els[i].className)) {
                classElements[j] = els[i];
                j++;
            }
        }
        return classElements;
    };

//设置cookie
    this.setAdCookie = function(N,V,Q){
        var L=new Date();
        var z=new Date(L.getTime()+Q*60000);

        //document.cookie=N+"="+escape(V)+";path=/;expires="+z.toGMTString()+";";
        document.cookie=N+"="+escape(V)+";path=/;domain=www.sina.com.cn;expires="+z.toGMTString()+";";

    };

//构造函数
    this.init = function(){
        var that = this;
        try{
            document.write('\
    <style type="text/css">\
    html{background:url(sina_101_2014_html_bg.jpg) repeat-x;}\
    body{background:url(' + CONFIG.activitySMBG + ') no-repeat 50% 0;padding-top:0;margin-top:0; background-color:#fff}\
    .topAD{background:#fff;}\
    .top-search-wrap .top-search-frame{background:#fff;height:58px}\
    #wrap{padding-top:0;margin-top:0;background:#fff}\
    .top-search-wrap{padding-top:0}\
    .top-search-frame{padding-top:15px}\
    #newyear2015Btncls{background:url(phbut_11.png) no-repeat;width:55px;height:22px;padding:0;margin:0;position:absolute;right:9px;bottom:49px;cursor:pointer;display:block;;z-index:2;}\
    #page{background:#fff;}\
    .body2016_newyear{background:url(//n.sinaimg.cn/default/1e20c22f/20170119/2017_01_20.jpg) no-repeat 50% 0 !important}\
    .channelHead{position:fixed!important;top:0!important;}\
    </style>\
    <div id="newyear2015TopBar" style="clear:both;width:1017px;height:140px;margin:0 auto;padding:0;overflow:hidden;position:relative;">\
    <div id="newyear2015Btncls" title="关闭背景" style=" ' +(CONFIG.isShowLeftLink ? '' : 'display:none;')+ ' "></div>\
     <a id="newyear2015TopBlank" style="clear:both;height:80px;width:930px;position:absolute;left:0;top:45px;line-height:0;font-size:0;overflow:hidden;" href="'+ CONFIG.leftLink +'" target="_blank" title="'+ CONFIG.activityChineseName +'"></a>\
    </div>\
    ');
            /* if(!window.XMLHttpRequest){
             document.body.className = "body2016_newyear";
             }*/

            pthis.$("newyear2015Btncls").onclick = function(){
                document.documentElement.style.background = '#fff';
                document.body.style.background = "#fff";
                that.getElementsByClass('top-search-wrap',document.body,'DIV')[0].style.paddingTop = "60px";

                document.getElementById("newyear2015TopBar").style.display = "none";
               document.getElementById("newyear2015TopBlank").style.display = "none";
                pthis.setAdCookie("2019guoqing",0,1000);
                /*if(!window.XMLHttpRequest){
                 document.body.className = "";
                 }*/
            };
        }catch(e){}
    }

    var cookie = pthis.getAdCookie("2019guoqing");
    cookie = cookie==""?1:cookie;
    if(cookie==1){pthis.init();}
}
})();
