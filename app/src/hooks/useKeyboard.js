import { useEffect } from "preact/hooks";

function useKeyboard(bindings) {
  useEffect(() => {
    function onKeyDown(e) {
      for (const binding of bindings) {
        if (binding.key !== e.key) continue;
        if (binding.meta && !(e.metaKey || e.ctrlKey)) continue;
        if (binding.when && !binding.when()) continue;
        e.preventDefault();
        binding.action();
        return;
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [bindings]);
}

export { useKeyboard };
