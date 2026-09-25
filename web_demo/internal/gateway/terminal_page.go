package gateway

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ttyd 1.7.7 fits xterm before its renderer and WebSocket preferences settle.
// A later fit sends a real PTY resize, so full-screen TUIs redraw at the size
// of the browser rather than staying at the initial 80x24 bottom-left block.
const terminalViewportEnhancement = `<style id="feam-terminal-viewport">
html,body{width:100%;height:100%;height:100dvh;margin:0;overflow:hidden}
#terminal-container{box-sizing:border-box;width:100vw!important;height:100vh!important;height:100dvh!important;max-width:none;margin:0;overflow:hidden}
#terminal-container .terminal{box-sizing:border-box;width:100%;height:100%;padding:8px}
</style><script id="feam-terminal-fit">
(function(){
  var pending=false, lastDpr=window.devicePixelRatio, dprWatch;
  function fontSize(){return window.innerWidth<640?16:(window.innerWidth>=1600?20:18)}
  function fit(){
    pending=false;
    var term=window.term;
    if(!term||typeof term.fit!=="function")return;
    var size=fontSize();
    if(term.options.fontSize!==size)term.options.fontSize=size;
    term.fit();
  }
  function schedule(){
    if(pending)return;
    pending=true;
    requestAnimationFrame(function(){requestAnimationFrame(fit)});
  }
  function watchDpr(){
    if(!window.matchMedia)return;
    if(dprWatch)dprWatch.removeEventListener("change",onDpr);
    dprWatch=window.matchMedia("(resolution: "+window.devicePixelRatio+"dppx)");
    dprWatch.addEventListener("change",onDpr);
  }
  function onDpr(){lastDpr=window.devicePixelRatio;watchDpr();schedule()}
  var tries=0, ready=setInterval(function(){
    if(window.term&&typeof window.term.fit==="function"){
      clearInterval(ready);
      schedule();
      setTimeout(schedule,300);
      setTimeout(schedule,1200);
    }else if(++tries>=200){clearInterval(ready)}
  },50);
  window.addEventListener("resize",schedule);
  if(window.visualViewport)window.visualViewport.addEventListener("resize",schedule);
  if(window.ResizeObserver){
    var container=document.getElementById("terminal-container");
    if(container)new ResizeObserver(schedule).observe(container);
  }
  watchDpr();
  setInterval(function(){if(window.devicePixelRatio!==lastDpr)onDpr()},1000);
})();
</script>`

const maxTerminalPageBytes = 4 << 20

func enhanceTerminalPage(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return errors.New("compressed terminal page")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTerminalPageBytes+1))
	if err != nil || len(body) > maxTerminalPageBytes {
		return errors.New("terminal page unavailable")
	}
	end := bytes.LastIndex(bytes.ToLower(body), []byte("</body>"))
	if end < 0 {
		return errors.New("unexpected terminal page")
	}
	page := make([]byte, 0, len(body)+len(terminalViewportEnhancement))
	page = append(page, body[:end]...)
	page = append(page, terminalViewportEnhancement...)
	page = append(page, body[end:]...)
	resp.Body = io.NopCloser(bytes.NewReader(page))
	resp.ContentLength = int64(len(page))
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(page)))
	return nil
}
