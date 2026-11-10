// fakeGl is a WebGL2 context that draws nothing and records every call, for tests of the renderer
// and the canvas that need no GPU. A create call returns a fresh object, so a test can tell which
// buffer or program a delete call freed. A constant reads as a number, and a parameter query
// answers true, so shaders compile and programs link.
export interface FakeGl {
  gl: WebGL2RenderingContext;
  calls: { name: string; args: unknown[]; ret?: unknown }[];
  count(name: string): number;
}

export function fakeGl(): FakeGl {
  const calls: { name: string; args: unknown[]; ret?: unknown }[] = [];
  let next = 1;
  const gl = new Proxy(
    {},
    {
      get(_t, prop) {
        if (typeof prop !== "string") return undefined;
        if (/^[A-Z0-9_]+$/.test(prop)) return 0x1000 + prop.length;
        if (prop === "canvas") return undefined;
        return (...args: unknown[]) => {
          const call: { name: string; args: unknown[]; ret?: unknown } = { name: prop, args };
          calls.push(call);
          if (prop.startsWith("create")) return (call.ret = { id: next++, kind: prop.slice(6) });
          if (prop === "getUniformLocation") return { uniform: args[1] };
          if (prop === "getAttribLocation") return 0;
          if (prop === "getShaderParameter" || prop === "getProgramParameter") return true;
          if (prop === "getExtension") return null;
          return undefined;
        };
      },
    },
  ) as WebGL2RenderingContext;
  return { gl, calls, count: (name) => calls.filter((c) => c.name === name).length };
}
