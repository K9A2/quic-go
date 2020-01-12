(function(g){var window=this;var Q5=function(a,b){var c="ytp-miniplayer-button-bottom-right";var d=g.Y?{F:"div",Y:["ytp-icon","ytp-icon-expand-watch-page"]}:{F:"svg",O:{height:"18px",version:"1.1",viewBox:"0 0 22 18",width:"22px"},L:[{F:"g",O:{fill:"none","fill-rule":"evenodd",stroke:"none","stroke-width":"1"},L:[{F:"g",O:{transform:"translate(-1.000000, -3.000000)"},L:[{F:"polygon",O:{points:"0 0 24 0 24 24 0 24"}},{F:"path",O:{d:"M19,7 L5,7 L5,17 L19,17 L19,7 Z M23,19 L23,4.98 C23,3.88 22.1,3 21,3 L3,3 C1.9,3 1,3.88 1,4.98 L1,19 C1,20.1 1.9,21 3,21 L21,21 C22.1,21 23,20.1 23,19 Z M21,19.02 L3,19.02 L3,4.97 L21,4.97 L21,19.02 Z",
fill:"#fff","fill-rule":"nonzero"}}]}]}]};var e="Open video page";g.T(a.P().experiments,"kevlar_miniplayer_expand_top")&&(c="ytp-miniplayer-button-top-left",d=g.Y?{F:"div",Y:["ytp-icon","ytp-icon-expand-miniplayer"]}:{F:"svg",O:{height:"24px",version:"1.1",viewBox:"0 0 24 24",width:"24px"},L:[{F:"g",O:{fill:"none","fill-rule":"evenodd",stroke:"none","stroke-width":"1"},L:[{F:"g",O:{transform:"translate(12.000000, 12.000000) scale(-1, 1) translate(-12.000000, -12.000000) "},L:[{F:"path",O:{d:"M19,19 L5,19 L5,5 L12,5 L12,3 L5,3 C3.89,3 3,3.9 3,5 L3,19 C3,20.1 3.89,21 5,21 L19,21 C20.1,21 21,20.1 21,19 L21,12 L19,12 L19,19 Z M14,3 L14,5 L17.59,5 L7.76,14.83 L9.17,16.24 L19,6.41 L19,10 L21,10 L21,3 L14,3 Z",
fill:"#fff","fill-rule":"nonzero"}}]}]}]},e="Expand");g.S.call(this,{F:"button",Y:["ytp-miniplayer-expand-watch-page-button","ytp-button",c],O:{title:"{{title}}","data-tooltip-target-id":"ytp-miniplayer-expand-watch-page-button"},L:[d]});this.w=a;this.ga("click",this.B,this);this.A("title",g.jO(a,e,"i"));g.ce(this,g.FO(b.Va(),this.element))},R5=function(a){g.S.call(this,{F:"div",
I:"ytp-miniplayer-ui"});this.player=a;this.K=!1;this.J=this.D=this.w=void 0;this.M(a,"minimized",this.BG);this.M(a,"onStateChange",this.DN)},S5=function(a){g.cR.call(this,a);
this.o=new R5(this.player);this.o.hide();g.FL(this.player,this.o.element,4);a.app.H.o&&(this.load(),g.M(a.getRootNode(),"ytp-player-minimized",!0))};
g.r(Q5,g.S);Q5.prototype.B=function(){this.w.ma("onExpandMiniplayer")};g.r(R5,g.S);g.h=R5.prototype;
g.h.show=function(){this.w=new g.Um(this.AG,null,this);this.w.start();if(!this.K){this.G=new g.BS(this.player,this);g.B(this,this.G);g.FL(this.player,this.G.element,4);this.G.D=.6;this.V=new g.wR(this.player);g.B(this,this.V);this.B=new g.S({F:"div",I:"ytp-miniplayer-scrim"});g.B(this,this.B);this.B.o(this.element);this.M(this.B.element,"click",this.oz);var a=new g.S({F:"button",Y:["ytp-miniplayer-close-button","ytp-button"],O:{"aria-label":"Close"},L:[g.xN()]});g.B(this,a);a.o(this.B.element);this.M(a.element,
"click",this.Jx);a=new Q5(this.player,this);g.B(this,a);a.o(this.B.element);this.C=new g.S({F:"div",I:"ytp-miniplayer-controls"});g.B(this,this.C);this.C.o(this.B.element);this.M(this.C.element,"click",this.oz);var b=new g.S({F:"div",I:"ytp-miniplayer-button-container"});g.B(this,b);b.o(this.C.element);a=new g.S({F:"div",I:"ytp-miniplayer-play-button-container"});g.B(this,a);a.o(this.C.element);var c=new g.S({F:"div",I:"ytp-miniplayer-button-container"});g.B(this,c);c.o(this.C.element);this.S=new g.lP(this.player,
this,!1);g.B(this,this.S);this.S.o(b.element);b=new g.iP(this.player,this);g.B(this,b);b.o(a.element);this.N=new g.lP(this.player,this,!0);g.B(this,this.N);this.N.o(c.element);this.J=new g.JQ(this.player,this);g.B(this,this.J);this.J.o(this.B.element);this.D=new g.uP(this.player,this);g.B(this,this.D);g.FL(this.player,this.D.element,4);this.H=new g.S({F:"div",I:"ytp-miniplayer-buttons"});g.B(this,this.H);g.FL(this.player,this.H.element,4);a=new g.S({F:"button",Y:["ytp-miniplayer-close-button","ytp-button"],
O:{"aria-label":"Close"},L:[g.xN()]});g.B(this,a);a.o(this.H.element);this.M(a.element,"click",this.Jx);a=new g.S({F:"button",Y:["ytp-miniplayer-replay-button","ytp-button"],O:{"aria-label":"Close"},L:[g.NN()]});g.B(this,a);a.o(this.H.element);this.M(a.element,"click",this.uL);this.M(this.player,"presentingplayerstatechange",this.CG);this.M(this.player,"appresize",this.ur);this.M(this.player,"fullscreentoggled",this.ur);this.ur();this.K=!0}0!=this.player.getPlayerState()&&g.S.prototype.show.call(this);
this.D.show();this.player.unloadModule("annotations_module")};
g.h.hide=function(){this.w&&(this.w.dispose(),this.w=void 0);g.S.prototype.hide.call(this);this.player.app.H.o||(this.K&&this.D.hide(),this.player.loadModule("annotations_module"))};
g.h.Z=function(){this.w&&(this.w.dispose(),this.w=void 0);g.S.prototype.Z.call(this)};
g.h.Jx=function(){this.player.stopVideo();this.player.ma("onCloseMiniplayer")};
g.h.uL=function(){this.player.playVideo()};
g.h.oz=function(a){if(a.target==this.B.element||a.target==this.C.element)g.T(this.player.P().experiments,"kevlar_miniplayer_play_pause_on_scrim")?g.YD(g.mL(this.player))?this.player.pauseVideo():this.player.playVideo():this.player.ma("onExpandMiniplayer")};
g.h.BG=function(){g.M(this.player.getRootNode(),"ytp-player-minimized",this.player.app.H.o)};
g.h.AG=function(){g.wP(this.D);this.J.w();this.w&&this.w.start()};
g.h.CG=function(a){g.W(a.state,32)&&this.G.hide()};
g.h.ur=function(){var a=this.D,b=g.nL(this.player).getPlayerSize().width;a.Da=0;a.D=b;a.H=!1;g.yP(a)};
g.h.DN=function(a){this.player.app.H.o&&(0==a?this.hide():this.show())};
g.h.Va=function(){return this.G};
g.h.Xb=function(){return!1};
g.h.Sd=function(){return!1};
g.h.yi=function(){return!1};
g.h.Us=function(){};
g.h.dh=function(){};
g.h.Fj=function(){};
g.h.yk=function(){return null};
g.h.tr=function(){return new g.hh(0,0,0,0)};
g.h.handleGlobalKeyDown=function(){return!1};
g.h.handleGlobalKeyUp=function(){return!1};
g.h.Ui=function(a,b,c,d,e){var f=0,k=d=0,l=g.Eh(a);if(b){c=g.fn(b,"ytp-prev-button")||g.fn(b,"ytp-next-button");var m=g.fn(b,"ytp-play-button"),n=g.fn(b,"ytp-miniplayer-expand-watch-page-button");c?f=k=12:m?(b=g.Bh(b,this.element),k=b.x,f=b.y-12):n&&(k=g.fn(b,"ytp-miniplayer-button-top-left"),f=g.Bh(b,this.element),b=g.Eh(b),k?(k=8,f=f.y+40):(k=f.x-l.width+b.width,f=f.y-20))}else k=c-l.width/2,d=25+(e||0);b=g.nL(this.player).getPlayerSize().width;e=f+(e||0);l=g.kd(k,0,b-l.width);e?(a.style.top=e+
"px",a.style.bottom=""):(a.style.top="",a.style.bottom=d+"px");a.style.left=l+"px"};
g.h.showControls=function(){};
g.h.vr=function(){};
g.h.Df=function(){};g.r(S5,g.cR);S5.prototype.create=function(){};
S5.prototype.gf=function(){return!1};
S5.prototype.load=function(){this.player.hideControls();this.o.show()};
S5.prototype.unload=function(){this.player.showControls();this.o.hide()};g.sR.miniplayer=S5;})(_yt_player);
