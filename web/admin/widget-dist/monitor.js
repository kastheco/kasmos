function lm(_) {
  return _ && _.__esModule && Object.prototype.hasOwnProperty.call(_, "default") ? _.default : _;
}
var sf = { exports: {} }, Ee = {};
/**
 * @license React
 * react-jsx-runtime.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var bd;
function tm() {
  if (bd) return Ee;
  bd = 1;
  var _ = Symbol.for("react.transitional.element"), D = Symbol.for("react.fragment");
  function U(d, L, yl) {
    var ul = null;
    if (yl !== void 0 && (ul = "" + yl), L.key !== void 0 && (ul = "" + L.key), "key" in L) {
      yl = {};
      for (var ml in L)
        ml !== "key" && (yl[ml] = L[ml]);
    } else yl = L;
    return L = yl.ref, {
      $$typeof: _,
      type: d,
      key: ul,
      ref: L !== void 0 ? L : null,
      props: yl
    };
  }
  return Ee.Fragment = D, Ee.jsx = U, Ee.jsxs = U, Ee;
}
var _d;
function am() {
  return _d || (_d = 1, sf.exports = tm()), sf.exports;
}
var p = am(), vf = { exports: {} }, Y = {};
/**
 * @license React
 * react.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var zd;
function um() {
  if (zd) return Y;
  zd = 1;
  var _ = Symbol.for("react.transitional.element"), D = Symbol.for("react.portal"), U = Symbol.for("react.fragment"), d = Symbol.for("react.strict_mode"), L = Symbol.for("react.profiler"), yl = Symbol.for("react.consumer"), ul = Symbol.for("react.context"), ml = Symbol.for("react.forward_ref"), A = Symbol.for("react.suspense"), T = Symbol.for("react.memo"), Z = Symbol.for("react.lazy"), x = Symbol.for("react.activity"), ll = Symbol.iterator;
  function Hl(v) {
    return v === null || typeof v != "object" ? null : (v = ll && v[ll] || v["@@iterator"], typeof v == "function" ? v : null);
  }
  var bl = {
    isMounted: function() {
      return !1;
    },
    enqueueForceUpdate: function() {
    },
    enqueueReplaceState: function() {
    },
    enqueueSetState: function() {
    }
  }, Rl = Object.assign, El = {};
  function rl(v, E, O) {
    this.props = v, this.context = E, this.refs = El, this.updater = O || bl;
  }
  rl.prototype.isReactComponent = {}, rl.prototype.setState = function(v, E) {
    if (typeof v != "object" && typeof v != "function" && v != null)
      throw Error(
        "takes an object of state variables to update or a function which returns an object of state variables."
      );
    this.updater.enqueueSetState(this, v, E, "setState");
  }, rl.prototype.forceUpdate = function(v) {
    this.updater.enqueueForceUpdate(this, v, "forceUpdate");
  };
  function Yl() {
  }
  Yl.prototype = rl.prototype;
  function Tl(v, E, O) {
    this.props = v, this.context = E, this.refs = El, this.updater = O || bl;
  }
  var kl = Tl.prototype = new Yl();
  kl.constructor = Tl, Rl(kl, rl.prototype), kl.isPureReactComponent = !0;
  var et = Array.isArray;
  function Gl() {
  }
  var V = { H: null, A: null, T: null, S: null }, Xl = Object.prototype.hasOwnProperty;
  function nt(v, E, O) {
    var H = O.ref;
    return {
      $$typeof: _,
      type: v,
      key: E,
      ref: H !== void 0 ? H : null,
      props: O
    };
  }
  function Bt(v, E) {
    return nt(v.type, E, v.props);
  }
  function ct(v) {
    return typeof v == "object" && v !== null && v.$$typeof === _;
  }
  function Ql(v) {
    var E = { "=": "=0", ":": "=2" };
    return "$" + v.replace(/[=:]/g, function(O) {
      return E[O];
    });
  }
  var Nt = /\/+/g;
  function j(v, E) {
    return typeof v == "object" && v !== null && v.key != null ? Ql("" + v.key) : E.toString(36);
  }
  function tl(v) {
    switch (v.status) {
      case "fulfilled":
        return v.value;
      case "rejected":
        throw v.reason;
      default:
        switch (typeof v.status == "string" ? v.then(Gl, Gl) : (v.status = "pending", v.then(
          function(E) {
            v.status === "pending" && (v.status = "fulfilled", v.value = E);
          },
          function(E) {
            v.status === "pending" && (v.status = "rejected", v.reason = E);
          }
        )), v.status) {
          case "fulfilled":
            return v.value;
          case "rejected":
            throw v.reason;
        }
    }
    throw v;
  }
  function S(v, E, O, H, G) {
    var K = typeof v;
    (K === "undefined" || K === "boolean") && (v = null);
    var al = !1;
    if (v === null) al = !0;
    else
      switch (K) {
        case "bigint":
        case "string":
        case "number":
          al = !0;
          break;
        case "object":
          switch (v.$$typeof) {
            case _:
            case D:
              al = !0;
              break;
            case Z:
              return al = v._init, S(
                al(v._payload),
                E,
                O,
                H,
                G
              );
          }
      }
    if (al)
      return G = G(v), al = H === "" ? "." + j(v, 0) : H, et(G) ? (O = "", al != null && (O = al.replace(Nt, "$&/") + "/"), S(G, E, O, "", function(Uu) {
        return Uu;
      })) : G != null && (ct(G) && (G = Bt(
        G,
        O + (G.key == null || v && v.key === G.key ? "" : ("" + G.key).replace(
          Nt,
          "$&/"
        ) + "/") + al
      )), E.push(G)), 1;
    al = 0;
    var Wl = H === "" ? "." : H + ":";
    if (et(v))
      for (var Al = 0; Al < v.length; Al++)
        H = v[Al], K = Wl + j(H, Al), al += S(
          H,
          E,
          O,
          K,
          G
        );
    else if (Al = Hl(v), typeof Al == "function")
      for (v = Al.call(v), Al = 0; !(H = v.next()).done; )
        H = H.value, K = Wl + j(H, Al++), al += S(
          H,
          E,
          O,
          K,
          G
        );
    else if (K === "object") {
      if (typeof v.then == "function")
        return S(
          tl(v),
          E,
          O,
          H,
          G
        );
      throw E = String(v), Error(
        "Objects are not valid as a React child (found: " + (E === "[object Object]" ? "object with keys {" + Object.keys(v).join(", ") + "}" : E) + "). If you meant to render a collection of children, use an array instead."
      );
    }
    return al;
  }
  function M(v, E, O) {
    if (v == null) return v;
    var H = [], G = 0;
    return S(v, H, "", "", function(K) {
      return E.call(O, K, G++);
    }), H;
  }
  function C(v) {
    if (v._status === -1) {
      var E = v._result;
      E = E(), E.then(
        function(O) {
          (v._status === 0 || v._status === -1) && (v._status = 1, v._result = O);
        },
        function(O) {
          (v._status === 0 || v._status === -1) && (v._status = 2, v._result = O);
        }
      ), v._status === -1 && (v._status = 0, v._result = E);
    }
    if (v._status === 1) return v._result.default;
    throw v._result;
  }
  var il = typeof reportError == "function" ? reportError : function(v) {
    if (typeof window == "object" && typeof window.ErrorEvent == "function") {
      var E = new window.ErrorEvent("error", {
        bubbles: !0,
        cancelable: !0,
        message: typeof v == "object" && v !== null && typeof v.message == "string" ? String(v.message) : String(v),
        error: v
      });
      if (!window.dispatchEvent(E)) return;
    } else if (typeof process == "object" && typeof process.emit == "function") {
      process.emit("uncaughtException", v);
      return;
    }
    console.error(v);
  }, ol = {
    map: M,
    forEach: function(v, E, O) {
      M(
        v,
        function() {
          E.apply(this, arguments);
        },
        O
      );
    },
    count: function(v) {
      var E = 0;
      return M(v, function() {
        E++;
      }), E;
    },
    toArray: function(v) {
      return M(v, function(E) {
        return E;
      }) || [];
    },
    only: function(v) {
      if (!ct(v))
        throw Error(
          "React.Children.only expected to receive a single React element child."
        );
      return v;
    }
  };
  return Y.Activity = x, Y.Children = ol, Y.Component = rl, Y.Fragment = U, Y.Profiler = L, Y.PureComponent = Tl, Y.StrictMode = d, Y.Suspense = A, Y.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = V, Y.__COMPILER_RUNTIME = {
    __proto__: null,
    c: function(v) {
      return V.H.useMemoCache(v);
    }
  }, Y.cache = function(v) {
    return function() {
      return v.apply(null, arguments);
    };
  }, Y.cacheSignal = function() {
    return null;
  }, Y.cloneElement = function(v, E, O) {
    if (v == null)
      throw Error(
        "The argument must be a React element, but you passed " + v + "."
      );
    var H = Rl({}, v.props), G = v.key;
    if (E != null)
      for (K in E.key !== void 0 && (G = "" + E.key), E)
        !Xl.call(E, K) || K === "key" || K === "__self" || K === "__source" || K === "ref" && E.ref === void 0 || (H[K] = E[K]);
    var K = arguments.length - 2;
    if (K === 1) H.children = O;
    else if (1 < K) {
      for (var al = Array(K), Wl = 0; Wl < K; Wl++)
        al[Wl] = arguments[Wl + 2];
      H.children = al;
    }
    return nt(v.type, G, H);
  }, Y.createContext = function(v) {
    return v = {
      $$typeof: ul,
      _currentValue: v,
      _currentValue2: v,
      _threadCount: 0,
      Provider: null,
      Consumer: null
    }, v.Provider = v, v.Consumer = {
      $$typeof: yl,
      _context: v
    }, v;
  }, Y.createElement = function(v, E, O) {
    var H, G = {}, K = null;
    if (E != null)
      for (H in E.key !== void 0 && (K = "" + E.key), E)
        Xl.call(E, H) && H !== "key" && H !== "__self" && H !== "__source" && (G[H] = E[H]);
    var al = arguments.length - 2;
    if (al === 1) G.children = O;
    else if (1 < al) {
      for (var Wl = Array(al), Al = 0; Al < al; Al++)
        Wl[Al] = arguments[Al + 2];
      G.children = Wl;
    }
    if (v && v.defaultProps)
      for (H in al = v.defaultProps, al)
        G[H] === void 0 && (G[H] = al[H]);
    return nt(v, K, G);
  }, Y.createRef = function() {
    return { current: null };
  }, Y.forwardRef = function(v) {
    return { $$typeof: ml, render: v };
  }, Y.isValidElement = ct, Y.lazy = function(v) {
    return {
      $$typeof: Z,
      _payload: { _status: -1, _result: v },
      _init: C
    };
  }, Y.memo = function(v, E) {
    return {
      $$typeof: T,
      type: v,
      compare: E === void 0 ? null : E
    };
  }, Y.startTransition = function(v) {
    var E = V.T, O = {};
    V.T = O;
    try {
      var H = v(), G = V.S;
      G !== null && G(O, H), typeof H == "object" && H !== null && typeof H.then == "function" && H.then(Gl, il);
    } catch (K) {
      il(K);
    } finally {
      E !== null && O.types !== null && (E.types = O.types), V.T = E;
    }
  }, Y.unstable_useCacheRefresh = function() {
    return V.H.useCacheRefresh();
  }, Y.use = function(v) {
    return V.H.use(v);
  }, Y.useActionState = function(v, E, O) {
    return V.H.useActionState(v, E, O);
  }, Y.useCallback = function(v, E) {
    return V.H.useCallback(v, E);
  }, Y.useContext = function(v) {
    return V.H.useContext(v);
  }, Y.useDebugValue = function() {
  }, Y.useDeferredValue = function(v, E) {
    return V.H.useDeferredValue(v, E);
  }, Y.useEffect = function(v, E) {
    return V.H.useEffect(v, E);
  }, Y.useEffectEvent = function(v) {
    return V.H.useEffectEvent(v);
  }, Y.useId = function() {
    return V.H.useId();
  }, Y.useImperativeHandle = function(v, E, O) {
    return V.H.useImperativeHandle(v, E, O);
  }, Y.useInsertionEffect = function(v, E) {
    return V.H.useInsertionEffect(v, E);
  }, Y.useLayoutEffect = function(v, E) {
    return V.H.useLayoutEffect(v, E);
  }, Y.useMemo = function(v, E) {
    return V.H.useMemo(v, E);
  }, Y.useOptimistic = function(v, E) {
    return V.H.useOptimistic(v, E);
  }, Y.useReducer = function(v, E, O) {
    return V.H.useReducer(v, E, O);
  }, Y.useRef = function(v) {
    return V.H.useRef(v);
  }, Y.useState = function(v) {
    return V.H.useState(v);
  }, Y.useSyncExternalStore = function(v, E, O) {
    return V.H.useSyncExternalStore(
      v,
      E,
      O
    );
  }, Y.useTransition = function() {
    return V.H.useTransition();
  }, Y.version = "19.2.4", Y;
}
var Ed;
function gf() {
  return Ed || (Ed = 1, vf.exports = um()), vf.exports;
}
var jl = gf();
const em = /* @__PURE__ */ lm(jl);
var of = { exports: {} }, Te = {}, df = { exports: {} }, yf = {};
/**
 * @license React
 * scheduler.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Td;
function nm() {
  return Td || (Td = 1, (function(_) {
    function D(S, M) {
      var C = S.length;
      S.push(M);
      l: for (; 0 < C; ) {
        var il = C - 1 >>> 1, ol = S[il];
        if (0 < L(ol, M))
          S[il] = M, S[C] = ol, C = il;
        else break l;
      }
    }
    function U(S) {
      return S.length === 0 ? null : S[0];
    }
    function d(S) {
      if (S.length === 0) return null;
      var M = S[0], C = S.pop();
      if (C !== M) {
        S[0] = C;
        l: for (var il = 0, ol = S.length, v = ol >>> 1; il < v; ) {
          var E = 2 * (il + 1) - 1, O = S[E], H = E + 1, G = S[H];
          if (0 > L(O, C))
            H < ol && 0 > L(G, O) ? (S[il] = G, S[H] = C, il = H) : (S[il] = O, S[E] = C, il = E);
          else if (H < ol && 0 > L(G, C))
            S[il] = G, S[H] = C, il = H;
          else break l;
        }
      }
      return M;
    }
    function L(S, M) {
      var C = S.sortIndex - M.sortIndex;
      return C !== 0 ? C : S.id - M.id;
    }
    if (_.unstable_now = void 0, typeof performance == "object" && typeof performance.now == "function") {
      var yl = performance;
      _.unstable_now = function() {
        return yl.now();
      };
    } else {
      var ul = Date, ml = ul.now();
      _.unstable_now = function() {
        return ul.now() - ml;
      };
    }
    var A = [], T = [], Z = 1, x = null, ll = 3, Hl = !1, bl = !1, Rl = !1, El = !1, rl = typeof setTimeout == "function" ? setTimeout : null, Yl = typeof clearTimeout == "function" ? clearTimeout : null, Tl = typeof setImmediate < "u" ? setImmediate : null;
    function kl(S) {
      for (var M = U(T); M !== null; ) {
        if (M.callback === null) d(T);
        else if (M.startTime <= S)
          d(T), M.sortIndex = M.expirationTime, D(A, M);
        else break;
        M = U(T);
      }
    }
    function et(S) {
      if (Rl = !1, kl(S), !bl)
        if (U(A) !== null)
          bl = !0, Gl || (Gl = !0, Ql());
        else {
          var M = U(T);
          M !== null && tl(et, M.startTime - S);
        }
    }
    var Gl = !1, V = -1, Xl = 5, nt = -1;
    function Bt() {
      return El ? !0 : !(_.unstable_now() - nt < Xl);
    }
    function ct() {
      if (El = !1, Gl) {
        var S = _.unstable_now();
        nt = S;
        var M = !0;
        try {
          l: {
            bl = !1, Rl && (Rl = !1, Yl(V), V = -1), Hl = !0;
            var C = ll;
            try {
              t: {
                for (kl(S), x = U(A); x !== null && !(x.expirationTime > S && Bt()); ) {
                  var il = x.callback;
                  if (typeof il == "function") {
                    x.callback = null, ll = x.priorityLevel;
                    var ol = il(
                      x.expirationTime <= S
                    );
                    if (S = _.unstable_now(), typeof ol == "function") {
                      x.callback = ol, kl(S), M = !0;
                      break t;
                    }
                    x === U(A) && d(A), kl(S);
                  } else d(A);
                  x = U(A);
                }
                if (x !== null) M = !0;
                else {
                  var v = U(T);
                  v !== null && tl(
                    et,
                    v.startTime - S
                  ), M = !1;
                }
              }
              break l;
            } finally {
              x = null, ll = C, Hl = !1;
            }
            M = void 0;
          }
        } finally {
          M ? Ql() : Gl = !1;
        }
      }
    }
    var Ql;
    if (typeof Tl == "function")
      Ql = function() {
        Tl(ct);
      };
    else if (typeof MessageChannel < "u") {
      var Nt = new MessageChannel(), j = Nt.port2;
      Nt.port1.onmessage = ct, Ql = function() {
        j.postMessage(null);
      };
    } else
      Ql = function() {
        rl(ct, 0);
      };
    function tl(S, M) {
      V = rl(function() {
        S(_.unstable_now());
      }, M);
    }
    _.unstable_IdlePriority = 5, _.unstable_ImmediatePriority = 1, _.unstable_LowPriority = 4, _.unstable_NormalPriority = 3, _.unstable_Profiling = null, _.unstable_UserBlockingPriority = 2, _.unstable_cancelCallback = function(S) {
      S.callback = null;
    }, _.unstable_forceFrameRate = function(S) {
      0 > S || 125 < S ? console.error(
        "forceFrameRate takes a positive int between 0 and 125, forcing frame rates higher than 125 fps is not supported"
      ) : Xl = 0 < S ? Math.floor(1e3 / S) : 5;
    }, _.unstable_getCurrentPriorityLevel = function() {
      return ll;
    }, _.unstable_next = function(S) {
      switch (ll) {
        case 1:
        case 2:
        case 3:
          var M = 3;
          break;
        default:
          M = ll;
      }
      var C = ll;
      ll = M;
      try {
        return S();
      } finally {
        ll = C;
      }
    }, _.unstable_requestPaint = function() {
      El = !0;
    }, _.unstable_runWithPriority = function(S, M) {
      switch (S) {
        case 1:
        case 2:
        case 3:
        case 4:
        case 5:
          break;
        default:
          S = 3;
      }
      var C = ll;
      ll = S;
      try {
        return M();
      } finally {
        ll = C;
      }
    }, _.unstable_scheduleCallback = function(S, M, C) {
      var il = _.unstable_now();
      switch (typeof C == "object" && C !== null ? (C = C.delay, C = typeof C == "number" && 0 < C ? il + C : il) : C = il, S) {
        case 1:
          var ol = -1;
          break;
        case 2:
          ol = 250;
          break;
        case 5:
          ol = 1073741823;
          break;
        case 4:
          ol = 1e4;
          break;
        default:
          ol = 5e3;
      }
      return ol = C + ol, S = {
        id: Z++,
        callback: M,
        priorityLevel: S,
        startTime: C,
        expirationTime: ol,
        sortIndex: -1
      }, C > il ? (S.sortIndex = C, D(T, S), U(A) === null && S === U(T) && (Rl ? (Yl(V), V = -1) : Rl = !0, tl(et, C - il))) : (S.sortIndex = ol, D(A, S), bl || Hl || (bl = !0, Gl || (Gl = !0, Ql()))), S;
    }, _.unstable_shouldYield = Bt, _.unstable_wrapCallback = function(S) {
      var M = ll;
      return function() {
        var C = ll;
        ll = M;
        try {
          return S.apply(this, arguments);
        } finally {
          ll = C;
        }
      };
    };
  })(yf)), yf;
}
var Ad;
function cm() {
  return Ad || (Ad = 1, df.exports = nm()), df.exports;
}
var mf = { exports: {} }, wl = {};
/**
 * @license React
 * react-dom.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var pd;
function im() {
  if (pd) return wl;
  pd = 1;
  var _ = gf();
  function D(A) {
    var T = "https://react.dev/errors/" + A;
    if (1 < arguments.length) {
      T += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var Z = 2; Z < arguments.length; Z++)
        T += "&args[]=" + encodeURIComponent(arguments[Z]);
    }
    return "Minified React error #" + A + "; visit " + T + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function U() {
  }
  var d = {
    d: {
      f: U,
      r: function() {
        throw Error(D(522));
      },
      D: U,
      C: U,
      L: U,
      m: U,
      X: U,
      S: U,
      M: U
    },
    p: 0,
    findDOMNode: null
  }, L = Symbol.for("react.portal");
  function yl(A, T, Z) {
    var x = 3 < arguments.length && arguments[3] !== void 0 ? arguments[3] : null;
    return {
      $$typeof: L,
      key: x == null ? null : "" + x,
      children: A,
      containerInfo: T,
      implementation: Z
    };
  }
  var ul = _.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE;
  function ml(A, T) {
    if (A === "font") return "";
    if (typeof T == "string")
      return T === "use-credentials" ? T : "";
  }
  return wl.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = d, wl.createPortal = function(A, T) {
    var Z = 2 < arguments.length && arguments[2] !== void 0 ? arguments[2] : null;
    if (!T || T.nodeType !== 1 && T.nodeType !== 9 && T.nodeType !== 11)
      throw Error(D(299));
    return yl(A, T, null, Z);
  }, wl.flushSync = function(A) {
    var T = ul.T, Z = d.p;
    try {
      if (ul.T = null, d.p = 2, A) return A();
    } finally {
      ul.T = T, d.p = Z, d.d.f();
    }
  }, wl.preconnect = function(A, T) {
    typeof A == "string" && (T ? (T = T.crossOrigin, T = typeof T == "string" ? T === "use-credentials" ? T : "" : void 0) : T = null, d.d.C(A, T));
  }, wl.prefetchDNS = function(A) {
    typeof A == "string" && d.d.D(A);
  }, wl.preinit = function(A, T) {
    if (typeof A == "string" && T && typeof T.as == "string") {
      var Z = T.as, x = ml(Z, T.crossOrigin), ll = typeof T.integrity == "string" ? T.integrity : void 0, Hl = typeof T.fetchPriority == "string" ? T.fetchPriority : void 0;
      Z === "style" ? d.d.S(
        A,
        typeof T.precedence == "string" ? T.precedence : void 0,
        {
          crossOrigin: x,
          integrity: ll,
          fetchPriority: Hl
        }
      ) : Z === "script" && d.d.X(A, {
        crossOrigin: x,
        integrity: ll,
        fetchPriority: Hl,
        nonce: typeof T.nonce == "string" ? T.nonce : void 0
      });
    }
  }, wl.preinitModule = function(A, T) {
    if (typeof A == "string")
      if (typeof T == "object" && T !== null) {
        if (T.as == null || T.as === "script") {
          var Z = ml(
            T.as,
            T.crossOrigin
          );
          d.d.M(A, {
            crossOrigin: Z,
            integrity: typeof T.integrity == "string" ? T.integrity : void 0,
            nonce: typeof T.nonce == "string" ? T.nonce : void 0
          });
        }
      } else T == null && d.d.M(A);
  }, wl.preload = function(A, T) {
    if (typeof A == "string" && typeof T == "object" && T !== null && typeof T.as == "string") {
      var Z = T.as, x = ml(Z, T.crossOrigin);
      d.d.L(A, Z, {
        crossOrigin: x,
        integrity: typeof T.integrity == "string" ? T.integrity : void 0,
        nonce: typeof T.nonce == "string" ? T.nonce : void 0,
        type: typeof T.type == "string" ? T.type : void 0,
        fetchPriority: typeof T.fetchPriority == "string" ? T.fetchPriority : void 0,
        referrerPolicy: typeof T.referrerPolicy == "string" ? T.referrerPolicy : void 0,
        imageSrcSet: typeof T.imageSrcSet == "string" ? T.imageSrcSet : void 0,
        imageSizes: typeof T.imageSizes == "string" ? T.imageSizes : void 0,
        media: typeof T.media == "string" ? T.media : void 0
      });
    }
  }, wl.preloadModule = function(A, T) {
    if (typeof A == "string")
      if (T) {
        var Z = ml(T.as, T.crossOrigin);
        d.d.m(A, {
          as: typeof T.as == "string" && T.as !== "script" ? T.as : void 0,
          crossOrigin: Z,
          integrity: typeof T.integrity == "string" ? T.integrity : void 0
        });
      } else d.d.m(A);
  }, wl.requestFormReset = function(A) {
    d.d.r(A);
  }, wl.unstable_batchedUpdates = function(A, T) {
    return A(T);
  }, wl.useFormState = function(A, T, Z) {
    return ul.H.useFormState(A, T, Z);
  }, wl.useFormStatus = function() {
    return ul.H.useHostTransitionStatus();
  }, wl.version = "19.2.4", wl;
}
var Md;
function fm() {
  if (Md) return mf.exports;
  Md = 1;
  function _() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(_);
      } catch (D) {
        console.error(D);
      }
  }
  return _(), mf.exports = im(), mf.exports;
}
/**
 * @license React
 * react-dom-client.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Od;
function sm() {
  if (Od) return Te;
  Od = 1;
  var _ = cm(), D = gf(), U = fm();
  function d(l) {
    var t = "https://react.dev/errors/" + l;
    if (1 < arguments.length) {
      t += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var a = 2; a < arguments.length; a++)
        t += "&args[]=" + encodeURIComponent(arguments[a]);
    }
    return "Minified React error #" + l + "; visit " + t + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function L(l) {
    return !(!l || l.nodeType !== 1 && l.nodeType !== 9 && l.nodeType !== 11);
  }
  function yl(l) {
    var t = l, a = l;
    if (l.alternate) for (; t.return; ) t = t.return;
    else {
      l = t;
      do
        t = l, (t.flags & 4098) !== 0 && (a = t.return), l = t.return;
      while (l);
    }
    return t.tag === 3 ? a : null;
  }
  function ul(l) {
    if (l.tag === 13) {
      var t = l.memoizedState;
      if (t === null && (l = l.alternate, l !== null && (t = l.memoizedState)), t !== null) return t.dehydrated;
    }
    return null;
  }
  function ml(l) {
    if (l.tag === 31) {
      var t = l.memoizedState;
      if (t === null && (l = l.alternate, l !== null && (t = l.memoizedState)), t !== null) return t.dehydrated;
    }
    return null;
  }
  function A(l) {
    if (yl(l) !== l)
      throw Error(d(188));
  }
  function T(l) {
    var t = l.alternate;
    if (!t) {
      if (t = yl(l), t === null) throw Error(d(188));
      return t !== l ? null : l;
    }
    for (var a = l, u = t; ; ) {
      var e = a.return;
      if (e === null) break;
      var n = e.alternate;
      if (n === null) {
        if (u = e.return, u !== null) {
          a = u;
          continue;
        }
        break;
      }
      if (e.child === n.child) {
        for (n = e.child; n; ) {
          if (n === a) return A(e), l;
          if (n === u) return A(e), t;
          n = n.sibling;
        }
        throw Error(d(188));
      }
      if (a.return !== u.return) a = e, u = n;
      else {
        for (var c = !1, i = e.child; i; ) {
          if (i === a) {
            c = !0, a = e, u = n;
            break;
          }
          if (i === u) {
            c = !0, u = e, a = n;
            break;
          }
          i = i.sibling;
        }
        if (!c) {
          for (i = n.child; i; ) {
            if (i === a) {
              c = !0, a = n, u = e;
              break;
            }
            if (i === u) {
              c = !0, u = n, a = e;
              break;
            }
            i = i.sibling;
          }
          if (!c) throw Error(d(189));
        }
      }
      if (a.alternate !== u) throw Error(d(190));
    }
    if (a.tag !== 3) throw Error(d(188));
    return a.stateNode.current === a ? l : t;
  }
  function Z(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l;
    for (l = l.child; l !== null; ) {
      if (t = Z(l), t !== null) return t;
      l = l.sibling;
    }
    return null;
  }
  var x = Object.assign, ll = Symbol.for("react.element"), Hl = Symbol.for("react.transitional.element"), bl = Symbol.for("react.portal"), Rl = Symbol.for("react.fragment"), El = Symbol.for("react.strict_mode"), rl = Symbol.for("react.profiler"), Yl = Symbol.for("react.consumer"), Tl = Symbol.for("react.context"), kl = Symbol.for("react.forward_ref"), et = Symbol.for("react.suspense"), Gl = Symbol.for("react.suspense_list"), V = Symbol.for("react.memo"), Xl = Symbol.for("react.lazy"), nt = Symbol.for("react.activity"), Bt = Symbol.for("react.memo_cache_sentinel"), ct = Symbol.iterator;
  function Ql(l) {
    return l === null || typeof l != "object" ? null : (l = ct && l[ct] || l["@@iterator"], typeof l == "function" ? l : null);
  }
  var Nt = Symbol.for("react.client.reference");
  function j(l) {
    if (l == null) return null;
    if (typeof l == "function")
      return l.$$typeof === Nt ? null : l.displayName || l.name || null;
    if (typeof l == "string") return l;
    switch (l) {
      case Rl:
        return "Fragment";
      case rl:
        return "Profiler";
      case El:
        return "StrictMode";
      case et:
        return "Suspense";
      case Gl:
        return "SuspenseList";
      case nt:
        return "Activity";
    }
    if (typeof l == "object")
      switch (l.$$typeof) {
        case bl:
          return "Portal";
        case Tl:
          return l.displayName || "Context";
        case Yl:
          return (l._context.displayName || "Context") + ".Consumer";
        case kl:
          var t = l.render;
          return l = l.displayName, l || (l = t.displayName || t.name || "", l = l !== "" ? "ForwardRef(" + l + ")" : "ForwardRef"), l;
        case V:
          return t = l.displayName || null, t !== null ? t : j(l.type) || "Memo";
        case Xl:
          t = l._payload, l = l._init;
          try {
            return j(l(t));
          } catch {
          }
      }
    return null;
  }
  var tl = Array.isArray, S = D.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, M = U.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, C = {
    pending: !1,
    data: null,
    method: null,
    action: null
  }, il = [], ol = -1;
  function v(l) {
    return { current: l };
  }
  function E(l) {
    0 > ol || (l.current = il[ol], il[ol] = null, ol--);
  }
  function O(l, t) {
    ol++, il[ol] = l.current, l.current = t;
  }
  var H = v(null), G = v(null), K = v(null), al = v(null);
  function Wl(l, t) {
    switch (O(K, t), O(G, l), O(H, null), t.nodeType) {
      case 9:
      case 11:
        l = (l = t.documentElement) && (l = l.namespaceURI) ? Zo(l) : 0;
        break;
      default:
        if (l = t.tagName, t = t.namespaceURI)
          t = Zo(t), l = Lo(t, l);
        else
          switch (l) {
            case "svg":
              l = 1;
              break;
            case "math":
              l = 2;
              break;
            default:
              l = 0;
          }
    }
    E(H), O(H, l);
  }
  function Al() {
    E(H), E(G), E(K);
  }
  function Uu(l) {
    l.memoizedState !== null && O(al, l);
    var t = H.current, a = Lo(t, l.type);
    t !== a && (O(G, l), O(H, a));
  }
  function pe(l) {
    G.current === l && (E(H), E(G)), al.current === l && (E(al), re._currentValue = C);
  }
  var Vn, Sf;
  function Ma(l) {
    if (Vn === void 0)
      try {
        throw Error();
      } catch (a) {
        var t = a.stack.trim().match(/\n( *(at )?)/);
        Vn = t && t[1] || "", Sf = -1 < a.stack.indexOf(`
    at`) ? " (<anonymous>)" : -1 < a.stack.indexOf("@") ? "@unknown:0:0" : "";
      }
    return `
` + Vn + l + Sf;
  }
  var Kn = !1;
  function Jn(l, t) {
    if (!l || Kn) return "";
    Kn = !0;
    var a = Error.prepareStackTrace;
    Error.prepareStackTrace = void 0;
    try {
      var u = {
        DetermineComponentFrameRoot: function() {
          try {
            if (t) {
              var z = function() {
                throw Error();
              };
              if (Object.defineProperty(z.prototype, "props", {
                set: function() {
                  throw Error();
                }
              }), typeof Reflect == "object" && Reflect.construct) {
                try {
                  Reflect.construct(z, []);
                } catch (g) {
                  var h = g;
                }
                Reflect.construct(l, [], z);
              } else {
                try {
                  z.call();
                } catch (g) {
                  h = g;
                }
                l.call(z.prototype);
              }
            } else {
              try {
                throw Error();
              } catch (g) {
                h = g;
              }
              (z = l()) && typeof z.catch == "function" && z.catch(function() {
              });
            }
          } catch (g) {
            if (g && h && typeof g.stack == "string")
              return [g.stack, h.stack];
          }
          return [null, null];
        }
      };
      u.DetermineComponentFrameRoot.displayName = "DetermineComponentFrameRoot";
      var e = Object.getOwnPropertyDescriptor(
        u.DetermineComponentFrameRoot,
        "name"
      );
      e && e.configurable && Object.defineProperty(
        u.DetermineComponentFrameRoot,
        "name",
        { value: "DetermineComponentFrameRoot" }
      );
      var n = u.DetermineComponentFrameRoot(), c = n[0], i = n[1];
      if (c && i) {
        var f = c.split(`
`), m = i.split(`
`);
        for (e = u = 0; u < f.length && !f[u].includes("DetermineComponentFrameRoot"); )
          u++;
        for (; e < m.length && !m[e].includes(
          "DetermineComponentFrameRoot"
        ); )
          e++;
        if (u === f.length || e === m.length)
          for (u = f.length - 1, e = m.length - 1; 1 <= u && 0 <= e && f[u] !== m[e]; )
            e--;
        for (; 1 <= u && 0 <= e; u--, e--)
          if (f[u] !== m[e]) {
            if (u !== 1 || e !== 1)
              do
                if (u--, e--, 0 > e || f[u] !== m[e]) {
                  var r = `
` + f[u].replace(" at new ", " at ");
                  return l.displayName && r.includes("<anonymous>") && (r = r.replace("<anonymous>", l.displayName)), r;
                }
              while (1 <= u && 0 <= e);
            break;
          }
      }
    } finally {
      Kn = !1, Error.prepareStackTrace = a;
    }
    return (a = l ? l.displayName || l.name : "") ? Ma(a) : "";
  }
  function jd(l, t) {
    switch (l.tag) {
      case 26:
      case 27:
      case 5:
        return Ma(l.type);
      case 16:
        return Ma("Lazy");
      case 13:
        return l.child !== t && t !== null ? Ma("Suspense Fallback") : Ma("Suspense");
      case 19:
        return Ma("SuspenseList");
      case 0:
      case 15:
        return Jn(l.type, !1);
      case 11:
        return Jn(l.type.render, !1);
      case 1:
        return Jn(l.type, !0);
      case 31:
        return Ma("Activity");
      default:
        return "";
    }
  }
  function rf(l) {
    try {
      var t = "", a = null;
      do
        t += jd(l, a), a = l, l = l.return;
      while (l);
      return t;
    } catch (u) {
      return `
Error generating stack: ` + u.message + `
` + u.stack;
    }
  }
  var wn = Object.prototype.hasOwnProperty, kn = _.unstable_scheduleCallback, Wn = _.unstable_cancelCallback, Hd = _.unstable_shouldYield, Rd = _.unstable_requestPaint, it = _.unstable_now, xd = _.unstable_getCurrentPriorityLevel, bf = _.unstable_ImmediatePriority, _f = _.unstable_UserBlockingPriority, Me = _.unstable_NormalPriority, qd = _.unstable_LowPriority, zf = _.unstable_IdlePriority, Bd = _.log, Cd = _.unstable_setDisableYieldValue, Nu = null, ft = null;
  function ta(l) {
    if (typeof Bd == "function" && Cd(l), ft && typeof ft.setStrictMode == "function")
      try {
        ft.setStrictMode(Nu, l);
      } catch {
      }
  }
  var st = Math.clz32 ? Math.clz32 : Xd, Yd = Math.log, Gd = Math.LN2;
  function Xd(l) {
    return l >>>= 0, l === 0 ? 32 : 31 - (Yd(l) / Gd | 0) | 0;
  }
  var Oe = 256, De = 262144, Ue = 4194304;
  function Oa(l) {
    var t = l & 42;
    if (t !== 0) return t;
    switch (l & -l) {
      case 1:
        return 1;
      case 2:
        return 2;
      case 4:
        return 4;
      case 8:
        return 8;
      case 16:
        return 16;
      case 32:
        return 32;
      case 64:
        return 64;
      case 128:
        return 128;
      case 256:
      case 512:
      case 1024:
      case 2048:
      case 4096:
      case 8192:
      case 16384:
      case 32768:
      case 65536:
      case 131072:
        return l & 261888;
      case 262144:
      case 524288:
      case 1048576:
      case 2097152:
        return l & 3932160;
      case 4194304:
      case 8388608:
      case 16777216:
      case 33554432:
        return l & 62914560;
      case 67108864:
        return 67108864;
      case 134217728:
        return 134217728;
      case 268435456:
        return 268435456;
      case 536870912:
        return 536870912;
      case 1073741824:
        return 0;
      default:
        return l;
    }
  }
  function Ne(l, t, a) {
    var u = l.pendingLanes;
    if (u === 0) return 0;
    var e = 0, n = l.suspendedLanes, c = l.pingedLanes;
    l = l.warmLanes;
    var i = u & 134217727;
    return i !== 0 ? (u = i & ~n, u !== 0 ? e = Oa(u) : (c &= i, c !== 0 ? e = Oa(c) : a || (a = i & ~l, a !== 0 && (e = Oa(a))))) : (i = u & ~n, i !== 0 ? e = Oa(i) : c !== 0 ? e = Oa(c) : a || (a = u & ~l, a !== 0 && (e = Oa(a)))), e === 0 ? 0 : t !== 0 && t !== e && (t & n) === 0 && (n = e & -e, a = t & -t, n >= a || n === 32 && (a & 4194048) !== 0) ? t : e;
  }
  function ju(l, t) {
    return (l.pendingLanes & ~(l.suspendedLanes & ~l.pingedLanes) & t) === 0;
  }
  function Qd(l, t) {
    switch (l) {
      case 1:
      case 2:
      case 4:
      case 8:
      case 64:
        return t + 250;
      case 16:
      case 32:
      case 128:
      case 256:
      case 512:
      case 1024:
      case 2048:
      case 4096:
      case 8192:
      case 16384:
      case 32768:
      case 65536:
      case 131072:
      case 262144:
      case 524288:
      case 1048576:
      case 2097152:
        return t + 5e3;
      case 4194304:
      case 8388608:
      case 16777216:
      case 33554432:
        return -1;
      case 67108864:
      case 134217728:
      case 268435456:
      case 536870912:
      case 1073741824:
        return -1;
      default:
        return -1;
    }
  }
  function Ef() {
    var l = Ue;
    return Ue <<= 1, (Ue & 62914560) === 0 && (Ue = 4194304), l;
  }
  function $n(l) {
    for (var t = [], a = 0; 31 > a; a++) t.push(l);
    return t;
  }
  function Hu(l, t) {
    l.pendingLanes |= t, t !== 268435456 && (l.suspendedLanes = 0, l.pingedLanes = 0, l.warmLanes = 0);
  }
  function Zd(l, t, a, u, e, n) {
    var c = l.pendingLanes;
    l.pendingLanes = a, l.suspendedLanes = 0, l.pingedLanes = 0, l.warmLanes = 0, l.expiredLanes &= a, l.entangledLanes &= a, l.errorRecoveryDisabledLanes &= a, l.shellSuspendCounter = 0;
    var i = l.entanglements, f = l.expirationTimes, m = l.hiddenUpdates;
    for (a = c & ~a; 0 < a; ) {
      var r = 31 - st(a), z = 1 << r;
      i[r] = 0, f[r] = -1;
      var h = m[r];
      if (h !== null)
        for (m[r] = null, r = 0; r < h.length; r++) {
          var g = h[r];
          g !== null && (g.lane &= -536870913);
        }
      a &= ~z;
    }
    u !== 0 && Tf(l, u, 0), n !== 0 && e === 0 && l.tag !== 0 && (l.suspendedLanes |= n & ~(c & ~t));
  }
  function Tf(l, t, a) {
    l.pendingLanes |= t, l.suspendedLanes &= ~t;
    var u = 31 - st(t);
    l.entangledLanes |= t, l.entanglements[u] = l.entanglements[u] | 1073741824 | a & 261930;
  }
  function Af(l, t) {
    var a = l.entangledLanes |= t;
    for (l = l.entanglements; a; ) {
      var u = 31 - st(a), e = 1 << u;
      e & t | l[u] & t && (l[u] |= t), a &= ~e;
    }
  }
  function pf(l, t) {
    var a = t & -t;
    return a = (a & 42) !== 0 ? 1 : Fn(a), (a & (l.suspendedLanes | t)) !== 0 ? 0 : a;
  }
  function Fn(l) {
    switch (l) {
      case 2:
        l = 1;
        break;
      case 8:
        l = 4;
        break;
      case 32:
        l = 16;
        break;
      case 256:
      case 512:
      case 1024:
      case 2048:
      case 4096:
      case 8192:
      case 16384:
      case 32768:
      case 65536:
      case 131072:
      case 262144:
      case 524288:
      case 1048576:
      case 2097152:
      case 4194304:
      case 8388608:
      case 16777216:
      case 33554432:
        l = 128;
        break;
      case 268435456:
        l = 134217728;
        break;
      default:
        l = 0;
    }
    return l;
  }
  function In(l) {
    return l &= -l, 2 < l ? 8 < l ? (l & 134217727) !== 0 ? 32 : 268435456 : 8 : 2;
  }
  function Mf() {
    var l = M.p;
    return l !== 0 ? l : (l = window.event, l === void 0 ? 32 : dd(l.type));
  }
  function Of(l, t) {
    var a = M.p;
    try {
      return M.p = l, t();
    } finally {
      M.p = a;
    }
  }
  var aa = Math.random().toString(36).slice(2), Zl = "__reactFiber$" + aa, Fl = "__reactProps$" + aa, Ka = "__reactContainer$" + aa, Pn = "__reactEvents$" + aa, Ld = "__reactListeners$" + aa, Vd = "__reactHandles$" + aa, Df = "__reactResources$" + aa, Ru = "__reactMarker$" + aa;
  function lc(l) {
    delete l[Zl], delete l[Fl], delete l[Pn], delete l[Ld], delete l[Vd];
  }
  function Ja(l) {
    var t = l[Zl];
    if (t) return t;
    for (var a = l.parentNode; a; ) {
      if (t = a[Ka] || a[Zl]) {
        if (a = t.alternate, t.child !== null || a !== null && a.child !== null)
          for (l = $o(l); l !== null; ) {
            if (a = l[Zl]) return a;
            l = $o(l);
          }
        return t;
      }
      l = a, a = l.parentNode;
    }
    return null;
  }
  function wa(l) {
    if (l = l[Zl] || l[Ka]) {
      var t = l.tag;
      if (t === 5 || t === 6 || t === 13 || t === 31 || t === 26 || t === 27 || t === 3)
        return l;
    }
    return null;
  }
  function xu(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l.stateNode;
    throw Error(d(33));
  }
  function ka(l) {
    var t = l[Df];
    return t || (t = l[Df] = { hoistableStyles: /* @__PURE__ */ new Map(), hoistableScripts: /* @__PURE__ */ new Map() }), t;
  }
  function ql(l) {
    l[Ru] = !0;
  }
  var Uf = /* @__PURE__ */ new Set(), Nf = {};
  function Da(l, t) {
    Wa(l, t), Wa(l + "Capture", t);
  }
  function Wa(l, t) {
    for (Nf[l] = t, l = 0; l < t.length; l++)
      Uf.add(t[l]);
  }
  var Kd = RegExp(
    "^[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD][:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\-.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]*$"
  ), jf = {}, Hf = {};
  function Jd(l) {
    return wn.call(Hf, l) ? !0 : wn.call(jf, l) ? !1 : Kd.test(l) ? Hf[l] = !0 : (jf[l] = !0, !1);
  }
  function je(l, t, a) {
    if (Jd(t))
      if (a === null) l.removeAttribute(t);
      else {
        switch (typeof a) {
          case "undefined":
          case "function":
          case "symbol":
            l.removeAttribute(t);
            return;
          case "boolean":
            var u = t.toLowerCase().slice(0, 5);
            if (u !== "data-" && u !== "aria-") {
              l.removeAttribute(t);
              return;
            }
        }
        l.setAttribute(t, "" + a);
      }
  }
  function He(l, t, a) {
    if (a === null) l.removeAttribute(t);
    else {
      switch (typeof a) {
        case "undefined":
        case "function":
        case "symbol":
        case "boolean":
          l.removeAttribute(t);
          return;
      }
      l.setAttribute(t, "" + a);
    }
  }
  function Ct(l, t, a, u) {
    if (u === null) l.removeAttribute(a);
    else {
      switch (typeof u) {
        case "undefined":
        case "function":
        case "symbol":
        case "boolean":
          l.removeAttribute(a);
          return;
      }
      l.setAttributeNS(t, a, "" + u);
    }
  }
  function St(l) {
    switch (typeof l) {
      case "bigint":
      case "boolean":
      case "number":
      case "string":
      case "undefined":
        return l;
      case "object":
        return l;
      default:
        return "";
    }
  }
  function Rf(l) {
    var t = l.type;
    return (l = l.nodeName) && l.toLowerCase() === "input" && (t === "checkbox" || t === "radio");
  }
  function wd(l, t, a) {
    var u = Object.getOwnPropertyDescriptor(
      l.constructor.prototype,
      t
    );
    if (!l.hasOwnProperty(t) && typeof u < "u" && typeof u.get == "function" && typeof u.set == "function") {
      var e = u.get, n = u.set;
      return Object.defineProperty(l, t, {
        configurable: !0,
        get: function() {
          return e.call(this);
        },
        set: function(c) {
          a = "" + c, n.call(this, c);
        }
      }), Object.defineProperty(l, t, {
        enumerable: u.enumerable
      }), {
        getValue: function() {
          return a;
        },
        setValue: function(c) {
          a = "" + c;
        },
        stopTracking: function() {
          l._valueTracker = null, delete l[t];
        }
      };
    }
  }
  function tc(l) {
    if (!l._valueTracker) {
      var t = Rf(l) ? "checked" : "value";
      l._valueTracker = wd(
        l,
        t,
        "" + l[t]
      );
    }
  }
  function xf(l) {
    if (!l) return !1;
    var t = l._valueTracker;
    if (!t) return !0;
    var a = t.getValue(), u = "";
    return l && (u = Rf(l) ? l.checked ? "true" : "false" : l.value), l = u, l !== a ? (t.setValue(l), !0) : !1;
  }
  function Re(l) {
    if (l = l || (typeof document < "u" ? document : void 0), typeof l > "u") return null;
    try {
      return l.activeElement || l.body;
    } catch {
      return l.body;
    }
  }
  var kd = /[\n"\\]/g;
  function rt(l) {
    return l.replace(
      kd,
      function(t) {
        return "\\" + t.charCodeAt(0).toString(16) + " ";
      }
    );
  }
  function ac(l, t, a, u, e, n, c, i) {
    l.name = "", c != null && typeof c != "function" && typeof c != "symbol" && typeof c != "boolean" ? l.type = c : l.removeAttribute("type"), t != null ? c === "number" ? (t === 0 && l.value === "" || l.value != t) && (l.value = "" + St(t)) : l.value !== "" + St(t) && (l.value = "" + St(t)) : c !== "submit" && c !== "reset" || l.removeAttribute("value"), t != null ? uc(l, c, St(t)) : a != null ? uc(l, c, St(a)) : u != null && l.removeAttribute("value"), e == null && n != null && (l.defaultChecked = !!n), e != null && (l.checked = e && typeof e != "function" && typeof e != "symbol"), i != null && typeof i != "function" && typeof i != "symbol" && typeof i != "boolean" ? l.name = "" + St(i) : l.removeAttribute("name");
  }
  function qf(l, t, a, u, e, n, c, i) {
    if (n != null && typeof n != "function" && typeof n != "symbol" && typeof n != "boolean" && (l.type = n), t != null || a != null) {
      if (!(n !== "submit" && n !== "reset" || t != null)) {
        tc(l);
        return;
      }
      a = a != null ? "" + St(a) : "", t = t != null ? "" + St(t) : a, i || t === l.value || (l.value = t), l.defaultValue = t;
    }
    u = u ?? e, u = typeof u != "function" && typeof u != "symbol" && !!u, l.checked = i ? l.checked : !!u, l.defaultChecked = !!u, c != null && typeof c != "function" && typeof c != "symbol" && typeof c != "boolean" && (l.name = c), tc(l);
  }
  function uc(l, t, a) {
    t === "number" && Re(l.ownerDocument) === l || l.defaultValue === "" + a || (l.defaultValue = "" + a);
  }
  function $a(l, t, a, u) {
    if (l = l.options, t) {
      t = {};
      for (var e = 0; e < a.length; e++)
        t["$" + a[e]] = !0;
      for (a = 0; a < l.length; a++)
        e = t.hasOwnProperty("$" + l[a].value), l[a].selected !== e && (l[a].selected = e), e && u && (l[a].defaultSelected = !0);
    } else {
      for (a = "" + St(a), t = null, e = 0; e < l.length; e++) {
        if (l[e].value === a) {
          l[e].selected = !0, u && (l[e].defaultSelected = !0);
          return;
        }
        t !== null || l[e].disabled || (t = l[e]);
      }
      t !== null && (t.selected = !0);
    }
  }
  function Bf(l, t, a) {
    if (t != null && (t = "" + St(t), t !== l.value && (l.value = t), a == null)) {
      l.defaultValue !== t && (l.defaultValue = t);
      return;
    }
    l.defaultValue = a != null ? "" + St(a) : "";
  }
  function Cf(l, t, a, u) {
    if (t == null) {
      if (u != null) {
        if (a != null) throw Error(d(92));
        if (tl(u)) {
          if (1 < u.length) throw Error(d(93));
          u = u[0];
        }
        a = u;
      }
      a == null && (a = ""), t = a;
    }
    a = St(t), l.defaultValue = a, u = l.textContent, u === a && u !== "" && u !== null && (l.value = u), tc(l);
  }
  function Fa(l, t) {
    if (t) {
      var a = l.firstChild;
      if (a && a === l.lastChild && a.nodeType === 3) {
        a.nodeValue = t;
        return;
      }
    }
    l.textContent = t;
  }
  var Wd = new Set(
    "animationIterationCount aspectRatio borderImageOutset borderImageSlice borderImageWidth boxFlex boxFlexGroup boxOrdinalGroup columnCount columns flex flexGrow flexPositive flexShrink flexNegative flexOrder gridArea gridRow gridRowEnd gridRowSpan gridRowStart gridColumn gridColumnEnd gridColumnSpan gridColumnStart fontWeight lineClamp lineHeight opacity order orphans scale tabSize widows zIndex zoom fillOpacity floodOpacity stopOpacity strokeDasharray strokeDashoffset strokeMiterlimit strokeOpacity strokeWidth MozAnimationIterationCount MozBoxFlex MozBoxFlexGroup MozLineClamp msAnimationIterationCount msFlex msZoom msFlexGrow msFlexNegative msFlexOrder msFlexPositive msFlexShrink msGridColumn msGridColumnSpan msGridRow msGridRowSpan WebkitAnimationIterationCount WebkitBoxFlex WebKitBoxFlexGroup WebkitBoxOrdinalGroup WebkitColumnCount WebkitColumns WebkitFlex WebkitFlexGrow WebkitFlexPositive WebkitFlexShrink WebkitLineClamp".split(
      " "
    )
  );
  function Yf(l, t, a) {
    var u = t.indexOf("--") === 0;
    a == null || typeof a == "boolean" || a === "" ? u ? l.setProperty(t, "") : t === "float" ? l.cssFloat = "" : l[t] = "" : u ? l.setProperty(t, a) : typeof a != "number" || a === 0 || Wd.has(t) ? t === "float" ? l.cssFloat = a : l[t] = ("" + a).trim() : l[t] = a + "px";
  }
  function Gf(l, t, a) {
    if (t != null && typeof t != "object")
      throw Error(d(62));
    if (l = l.style, a != null) {
      for (var u in a)
        !a.hasOwnProperty(u) || t != null && t.hasOwnProperty(u) || (u.indexOf("--") === 0 ? l.setProperty(u, "") : u === "float" ? l.cssFloat = "" : l[u] = "");
      for (var e in t)
        u = t[e], t.hasOwnProperty(e) && a[e] !== u && Yf(l, e, u);
    } else
      for (var n in t)
        t.hasOwnProperty(n) && Yf(l, n, t[n]);
  }
  function ec(l) {
    if (l.indexOf("-") === -1) return !1;
    switch (l) {
      case "annotation-xml":
      case "color-profile":
      case "font-face":
      case "font-face-src":
      case "font-face-uri":
      case "font-face-format":
      case "font-face-name":
      case "missing-glyph":
        return !1;
      default:
        return !0;
    }
  }
  var $d = /* @__PURE__ */ new Map([
    ["acceptCharset", "accept-charset"],
    ["htmlFor", "for"],
    ["httpEquiv", "http-equiv"],
    ["crossOrigin", "crossorigin"],
    ["accentHeight", "accent-height"],
    ["alignmentBaseline", "alignment-baseline"],
    ["arabicForm", "arabic-form"],
    ["baselineShift", "baseline-shift"],
    ["capHeight", "cap-height"],
    ["clipPath", "clip-path"],
    ["clipRule", "clip-rule"],
    ["colorInterpolation", "color-interpolation"],
    ["colorInterpolationFilters", "color-interpolation-filters"],
    ["colorProfile", "color-profile"],
    ["colorRendering", "color-rendering"],
    ["dominantBaseline", "dominant-baseline"],
    ["enableBackground", "enable-background"],
    ["fillOpacity", "fill-opacity"],
    ["fillRule", "fill-rule"],
    ["floodColor", "flood-color"],
    ["floodOpacity", "flood-opacity"],
    ["fontFamily", "font-family"],
    ["fontSize", "font-size"],
    ["fontSizeAdjust", "font-size-adjust"],
    ["fontStretch", "font-stretch"],
    ["fontStyle", "font-style"],
    ["fontVariant", "font-variant"],
    ["fontWeight", "font-weight"],
    ["glyphName", "glyph-name"],
    ["glyphOrientationHorizontal", "glyph-orientation-horizontal"],
    ["glyphOrientationVertical", "glyph-orientation-vertical"],
    ["horizAdvX", "horiz-adv-x"],
    ["horizOriginX", "horiz-origin-x"],
    ["imageRendering", "image-rendering"],
    ["letterSpacing", "letter-spacing"],
    ["lightingColor", "lighting-color"],
    ["markerEnd", "marker-end"],
    ["markerMid", "marker-mid"],
    ["markerStart", "marker-start"],
    ["overlinePosition", "overline-position"],
    ["overlineThickness", "overline-thickness"],
    ["paintOrder", "paint-order"],
    ["panose-1", "panose-1"],
    ["pointerEvents", "pointer-events"],
    ["renderingIntent", "rendering-intent"],
    ["shapeRendering", "shape-rendering"],
    ["stopColor", "stop-color"],
    ["stopOpacity", "stop-opacity"],
    ["strikethroughPosition", "strikethrough-position"],
    ["strikethroughThickness", "strikethrough-thickness"],
    ["strokeDasharray", "stroke-dasharray"],
    ["strokeDashoffset", "stroke-dashoffset"],
    ["strokeLinecap", "stroke-linecap"],
    ["strokeLinejoin", "stroke-linejoin"],
    ["strokeMiterlimit", "stroke-miterlimit"],
    ["strokeOpacity", "stroke-opacity"],
    ["strokeWidth", "stroke-width"],
    ["textAnchor", "text-anchor"],
    ["textDecoration", "text-decoration"],
    ["textRendering", "text-rendering"],
    ["transformOrigin", "transform-origin"],
    ["underlinePosition", "underline-position"],
    ["underlineThickness", "underline-thickness"],
    ["unicodeBidi", "unicode-bidi"],
    ["unicodeRange", "unicode-range"],
    ["unitsPerEm", "units-per-em"],
    ["vAlphabetic", "v-alphabetic"],
    ["vHanging", "v-hanging"],
    ["vIdeographic", "v-ideographic"],
    ["vMathematical", "v-mathematical"],
    ["vectorEffect", "vector-effect"],
    ["vertAdvY", "vert-adv-y"],
    ["vertOriginX", "vert-origin-x"],
    ["vertOriginY", "vert-origin-y"],
    ["wordSpacing", "word-spacing"],
    ["writingMode", "writing-mode"],
    ["xmlnsXlink", "xmlns:xlink"],
    ["xHeight", "x-height"]
  ]), Fd = /^[\u0000-\u001F ]*j[\r\n\t]*a[\r\n\t]*v[\r\n\t]*a[\r\n\t]*s[\r\n\t]*c[\r\n\t]*r[\r\n\t]*i[\r\n\t]*p[\r\n\t]*t[\r\n\t]*:/i;
  function xe(l) {
    return Fd.test("" + l) ? "javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')" : l;
  }
  function Yt() {
  }
  var nc = null;
  function cc(l) {
    return l = l.target || l.srcElement || window, l.correspondingUseElement && (l = l.correspondingUseElement), l.nodeType === 3 ? l.parentNode : l;
  }
  var Ia = null, Pa = null;
  function Xf(l) {
    var t = wa(l);
    if (t && (l = t.stateNode)) {
      var a = l[Fl] || null;
      l: switch (l = t.stateNode, t.type) {
        case "input":
          if (ac(
            l,
            a.value,
            a.defaultValue,
            a.defaultValue,
            a.checked,
            a.defaultChecked,
            a.type,
            a.name
          ), t = a.name, a.type === "radio" && t != null) {
            for (a = l; a.parentNode; ) a = a.parentNode;
            for (a = a.querySelectorAll(
              'input[name="' + rt(
                "" + t
              ) + '"][type="radio"]'
            ), t = 0; t < a.length; t++) {
              var u = a[t];
              if (u !== l && u.form === l.form) {
                var e = u[Fl] || null;
                if (!e) throw Error(d(90));
                ac(
                  u,
                  e.value,
                  e.defaultValue,
                  e.defaultValue,
                  e.checked,
                  e.defaultChecked,
                  e.type,
                  e.name
                );
              }
            }
            for (t = 0; t < a.length; t++)
              u = a[t], u.form === l.form && xf(u);
          }
          break l;
        case "textarea":
          Bf(l, a.value, a.defaultValue);
          break l;
        case "select":
          t = a.value, t != null && $a(l, !!a.multiple, t, !1);
      }
    }
  }
  var ic = !1;
  function Qf(l, t, a) {
    if (ic) return l(t, a);
    ic = !0;
    try {
      var u = l(t);
      return u;
    } finally {
      if (ic = !1, (Ia !== null || Pa !== null) && (En(), Ia && (t = Ia, l = Pa, Pa = Ia = null, Xf(t), l)))
        for (t = 0; t < l.length; t++) Xf(l[t]);
    }
  }
  function qu(l, t) {
    var a = l.stateNode;
    if (a === null) return null;
    var u = a[Fl] || null;
    if (u === null) return null;
    a = u[t];
    l: switch (t) {
      case "onClick":
      case "onClickCapture":
      case "onDoubleClick":
      case "onDoubleClickCapture":
      case "onMouseDown":
      case "onMouseDownCapture":
      case "onMouseMove":
      case "onMouseMoveCapture":
      case "onMouseUp":
      case "onMouseUpCapture":
      case "onMouseEnter":
        (u = !u.disabled) || (l = l.type, u = !(l === "button" || l === "input" || l === "select" || l === "textarea")), l = !u;
        break l;
      default:
        l = !1;
    }
    if (l) return null;
    if (a && typeof a != "function")
      throw Error(
        d(231, t, typeof a)
      );
    return a;
  }
  var Gt = !(typeof window > "u" || typeof window.document > "u" || typeof window.document.createElement > "u"), fc = !1;
  if (Gt)
    try {
      var Bu = {};
      Object.defineProperty(Bu, "passive", {
        get: function() {
          fc = !0;
        }
      }), window.addEventListener("test", Bu, Bu), window.removeEventListener("test", Bu, Bu);
    } catch {
      fc = !1;
    }
  var ua = null, sc = null, qe = null;
  function Zf() {
    if (qe) return qe;
    var l, t = sc, a = t.length, u, e = "value" in ua ? ua.value : ua.textContent, n = e.length;
    for (l = 0; l < a && t[l] === e[l]; l++) ;
    var c = a - l;
    for (u = 1; u <= c && t[a - u] === e[n - u]; u++) ;
    return qe = e.slice(l, 1 < u ? 1 - u : void 0);
  }
  function Be(l) {
    var t = l.keyCode;
    return "charCode" in l ? (l = l.charCode, l === 0 && t === 13 && (l = 13)) : l = t, l === 10 && (l = 13), 32 <= l || l === 13 ? l : 0;
  }
  function Ce() {
    return !0;
  }
  function Lf() {
    return !1;
  }
  function Il(l) {
    function t(a, u, e, n, c) {
      this._reactName = a, this._targetInst = e, this.type = u, this.nativeEvent = n, this.target = c, this.currentTarget = null;
      for (var i in l)
        l.hasOwnProperty(i) && (a = l[i], this[i] = a ? a(n) : n[i]);
      return this.isDefaultPrevented = (n.defaultPrevented != null ? n.defaultPrevented : n.returnValue === !1) ? Ce : Lf, this.isPropagationStopped = Lf, this;
    }
    return x(t.prototype, {
      preventDefault: function() {
        this.defaultPrevented = !0;
        var a = this.nativeEvent;
        a && (a.preventDefault ? a.preventDefault() : typeof a.returnValue != "unknown" && (a.returnValue = !1), this.isDefaultPrevented = Ce);
      },
      stopPropagation: function() {
        var a = this.nativeEvent;
        a && (a.stopPropagation ? a.stopPropagation() : typeof a.cancelBubble != "unknown" && (a.cancelBubble = !0), this.isPropagationStopped = Ce);
      },
      persist: function() {
      },
      isPersistent: Ce
    }), t;
  }
  var Ua = {
    eventPhase: 0,
    bubbles: 0,
    cancelable: 0,
    timeStamp: function(l) {
      return l.timeStamp || Date.now();
    },
    defaultPrevented: 0,
    isTrusted: 0
  }, Ye = Il(Ua), Cu = x({}, Ua, { view: 0, detail: 0 }), Id = Il(Cu), vc, oc, Yu, Ge = x({}, Cu, {
    screenX: 0,
    screenY: 0,
    clientX: 0,
    clientY: 0,
    pageX: 0,
    pageY: 0,
    ctrlKey: 0,
    shiftKey: 0,
    altKey: 0,
    metaKey: 0,
    getModifierState: yc,
    button: 0,
    buttons: 0,
    relatedTarget: function(l) {
      return l.relatedTarget === void 0 ? l.fromElement === l.srcElement ? l.toElement : l.fromElement : l.relatedTarget;
    },
    movementX: function(l) {
      return "movementX" in l ? l.movementX : (l !== Yu && (Yu && l.type === "mousemove" ? (vc = l.screenX - Yu.screenX, oc = l.screenY - Yu.screenY) : oc = vc = 0, Yu = l), vc);
    },
    movementY: function(l) {
      return "movementY" in l ? l.movementY : oc;
    }
  }), Vf = Il(Ge), Pd = x({}, Ge, { dataTransfer: 0 }), l0 = Il(Pd), t0 = x({}, Cu, { relatedTarget: 0 }), dc = Il(t0), a0 = x({}, Ua, {
    animationName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), u0 = Il(a0), e0 = x({}, Ua, {
    clipboardData: function(l) {
      return "clipboardData" in l ? l.clipboardData : window.clipboardData;
    }
  }), n0 = Il(e0), c0 = x({}, Ua, { data: 0 }), Kf = Il(c0), i0 = {
    Esc: "Escape",
    Spacebar: " ",
    Left: "ArrowLeft",
    Up: "ArrowUp",
    Right: "ArrowRight",
    Down: "ArrowDown",
    Del: "Delete",
    Win: "OS",
    Menu: "ContextMenu",
    Apps: "ContextMenu",
    Scroll: "ScrollLock",
    MozPrintableKey: "Unidentified"
  }, f0 = {
    8: "Backspace",
    9: "Tab",
    12: "Clear",
    13: "Enter",
    16: "Shift",
    17: "Control",
    18: "Alt",
    19: "Pause",
    20: "CapsLock",
    27: "Escape",
    32: " ",
    33: "PageUp",
    34: "PageDown",
    35: "End",
    36: "Home",
    37: "ArrowLeft",
    38: "ArrowUp",
    39: "ArrowRight",
    40: "ArrowDown",
    45: "Insert",
    46: "Delete",
    112: "F1",
    113: "F2",
    114: "F3",
    115: "F4",
    116: "F5",
    117: "F6",
    118: "F7",
    119: "F8",
    120: "F9",
    121: "F10",
    122: "F11",
    123: "F12",
    144: "NumLock",
    145: "ScrollLock",
    224: "Meta"
  }, s0 = {
    Alt: "altKey",
    Control: "ctrlKey",
    Meta: "metaKey",
    Shift: "shiftKey"
  };
  function v0(l) {
    var t = this.nativeEvent;
    return t.getModifierState ? t.getModifierState(l) : (l = s0[l]) ? !!t[l] : !1;
  }
  function yc() {
    return v0;
  }
  var o0 = x({}, Cu, {
    key: function(l) {
      if (l.key) {
        var t = i0[l.key] || l.key;
        if (t !== "Unidentified") return t;
      }
      return l.type === "keypress" ? (l = Be(l), l === 13 ? "Enter" : String.fromCharCode(l)) : l.type === "keydown" || l.type === "keyup" ? f0[l.keyCode] || "Unidentified" : "";
    },
    code: 0,
    location: 0,
    ctrlKey: 0,
    shiftKey: 0,
    altKey: 0,
    metaKey: 0,
    repeat: 0,
    locale: 0,
    getModifierState: yc,
    charCode: function(l) {
      return l.type === "keypress" ? Be(l) : 0;
    },
    keyCode: function(l) {
      return l.type === "keydown" || l.type === "keyup" ? l.keyCode : 0;
    },
    which: function(l) {
      return l.type === "keypress" ? Be(l) : l.type === "keydown" || l.type === "keyup" ? l.keyCode : 0;
    }
  }), d0 = Il(o0), y0 = x({}, Ge, {
    pointerId: 0,
    width: 0,
    height: 0,
    pressure: 0,
    tangentialPressure: 0,
    tiltX: 0,
    tiltY: 0,
    twist: 0,
    pointerType: 0,
    isPrimary: 0
  }), Jf = Il(y0), m0 = x({}, Cu, {
    touches: 0,
    targetTouches: 0,
    changedTouches: 0,
    altKey: 0,
    metaKey: 0,
    ctrlKey: 0,
    shiftKey: 0,
    getModifierState: yc
  }), h0 = Il(m0), g0 = x({}, Ua, {
    propertyName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), S0 = Il(g0), r0 = x({}, Ge, {
    deltaX: function(l) {
      return "deltaX" in l ? l.deltaX : "wheelDeltaX" in l ? -l.wheelDeltaX : 0;
    },
    deltaY: function(l) {
      return "deltaY" in l ? l.deltaY : "wheelDeltaY" in l ? -l.wheelDeltaY : "wheelDelta" in l ? -l.wheelDelta : 0;
    },
    deltaZ: 0,
    deltaMode: 0
  }), b0 = Il(r0), _0 = x({}, Ua, {
    newState: 0,
    oldState: 0
  }), z0 = Il(_0), E0 = [9, 13, 27, 32], mc = Gt && "CompositionEvent" in window, Gu = null;
  Gt && "documentMode" in document && (Gu = document.documentMode);
  var T0 = Gt && "TextEvent" in window && !Gu, wf = Gt && (!mc || Gu && 8 < Gu && 11 >= Gu), kf = " ", Wf = !1;
  function $f(l, t) {
    switch (l) {
      case "keyup":
        return E0.indexOf(t.keyCode) !== -1;
      case "keydown":
        return t.keyCode !== 229;
      case "keypress":
      case "mousedown":
      case "focusout":
        return !0;
      default:
        return !1;
    }
  }
  function Ff(l) {
    return l = l.detail, typeof l == "object" && "data" in l ? l.data : null;
  }
  var lu = !1;
  function A0(l, t) {
    switch (l) {
      case "compositionend":
        return Ff(t);
      case "keypress":
        return t.which !== 32 ? null : (Wf = !0, kf);
      case "textInput":
        return l = t.data, l === kf && Wf ? null : l;
      default:
        return null;
    }
  }
  function p0(l, t) {
    if (lu)
      return l === "compositionend" || !mc && $f(l, t) ? (l = Zf(), qe = sc = ua = null, lu = !1, l) : null;
    switch (l) {
      case "paste":
        return null;
      case "keypress":
        if (!(t.ctrlKey || t.altKey || t.metaKey) || t.ctrlKey && t.altKey) {
          if (t.char && 1 < t.char.length)
            return t.char;
          if (t.which) return String.fromCharCode(t.which);
        }
        return null;
      case "compositionend":
        return wf && t.locale !== "ko" ? null : t.data;
      default:
        return null;
    }
  }
  var M0 = {
    color: !0,
    date: !0,
    datetime: !0,
    "datetime-local": !0,
    email: !0,
    month: !0,
    number: !0,
    password: !0,
    range: !0,
    search: !0,
    tel: !0,
    text: !0,
    time: !0,
    url: !0,
    week: !0
  };
  function If(l) {
    var t = l && l.nodeName && l.nodeName.toLowerCase();
    return t === "input" ? !!M0[l.type] : t === "textarea";
  }
  function Pf(l, t, a, u) {
    Ia ? Pa ? Pa.push(u) : Pa = [u] : Ia = u, t = Un(t, "onChange"), 0 < t.length && (a = new Ye(
      "onChange",
      "change",
      null,
      a,
      u
    ), l.push({ event: a, listeners: t }));
  }
  var Xu = null, Qu = null;
  function O0(l) {
    Bo(l, 0);
  }
  function Xe(l) {
    var t = xu(l);
    if (xf(t)) return l;
  }
  function ls(l, t) {
    if (l === "change") return t;
  }
  var ts = !1;
  if (Gt) {
    var hc;
    if (Gt) {
      var gc = "oninput" in document;
      if (!gc) {
        var as = document.createElement("div");
        as.setAttribute("oninput", "return;"), gc = typeof as.oninput == "function";
      }
      hc = gc;
    } else hc = !1;
    ts = hc && (!document.documentMode || 9 < document.documentMode);
  }
  function us() {
    Xu && (Xu.detachEvent("onpropertychange", es), Qu = Xu = null);
  }
  function es(l) {
    if (l.propertyName === "value" && Xe(Qu)) {
      var t = [];
      Pf(
        t,
        Qu,
        l,
        cc(l)
      ), Qf(O0, t);
    }
  }
  function D0(l, t, a) {
    l === "focusin" ? (us(), Xu = t, Qu = a, Xu.attachEvent("onpropertychange", es)) : l === "focusout" && us();
  }
  function U0(l) {
    if (l === "selectionchange" || l === "keyup" || l === "keydown")
      return Xe(Qu);
  }
  function N0(l, t) {
    if (l === "click") return Xe(t);
  }
  function j0(l, t) {
    if (l === "input" || l === "change")
      return Xe(t);
  }
  function H0(l, t) {
    return l === t && (l !== 0 || 1 / l === 1 / t) || l !== l && t !== t;
  }
  var vt = typeof Object.is == "function" ? Object.is : H0;
  function Zu(l, t) {
    if (vt(l, t)) return !0;
    if (typeof l != "object" || l === null || typeof t != "object" || t === null)
      return !1;
    var a = Object.keys(l), u = Object.keys(t);
    if (a.length !== u.length) return !1;
    for (u = 0; u < a.length; u++) {
      var e = a[u];
      if (!wn.call(t, e) || !vt(l[e], t[e]))
        return !1;
    }
    return !0;
  }
  function ns(l) {
    for (; l && l.firstChild; ) l = l.firstChild;
    return l;
  }
  function cs(l, t) {
    var a = ns(l);
    l = 0;
    for (var u; a; ) {
      if (a.nodeType === 3) {
        if (u = l + a.textContent.length, l <= t && u >= t)
          return { node: a, offset: t - l };
        l = u;
      }
      l: {
        for (; a; ) {
          if (a.nextSibling) {
            a = a.nextSibling;
            break l;
          }
          a = a.parentNode;
        }
        a = void 0;
      }
      a = ns(a);
    }
  }
  function is(l, t) {
    return l && t ? l === t ? !0 : l && l.nodeType === 3 ? !1 : t && t.nodeType === 3 ? is(l, t.parentNode) : "contains" in l ? l.contains(t) : l.compareDocumentPosition ? !!(l.compareDocumentPosition(t) & 16) : !1 : !1;
  }
  function fs(l) {
    l = l != null && l.ownerDocument != null && l.ownerDocument.defaultView != null ? l.ownerDocument.defaultView : window;
    for (var t = Re(l.document); t instanceof l.HTMLIFrameElement; ) {
      try {
        var a = typeof t.contentWindow.location.href == "string";
      } catch {
        a = !1;
      }
      if (a) l = t.contentWindow;
      else break;
      t = Re(l.document);
    }
    return t;
  }
  function Sc(l) {
    var t = l && l.nodeName && l.nodeName.toLowerCase();
    return t && (t === "input" && (l.type === "text" || l.type === "search" || l.type === "tel" || l.type === "url" || l.type === "password") || t === "textarea" || l.contentEditable === "true");
  }
  var R0 = Gt && "documentMode" in document && 11 >= document.documentMode, tu = null, rc = null, Lu = null, bc = !1;
  function ss(l, t, a) {
    var u = a.window === a ? a.document : a.nodeType === 9 ? a : a.ownerDocument;
    bc || tu == null || tu !== Re(u) || (u = tu, "selectionStart" in u && Sc(u) ? u = { start: u.selectionStart, end: u.selectionEnd } : (u = (u.ownerDocument && u.ownerDocument.defaultView || window).getSelection(), u = {
      anchorNode: u.anchorNode,
      anchorOffset: u.anchorOffset,
      focusNode: u.focusNode,
      focusOffset: u.focusOffset
    }), Lu && Zu(Lu, u) || (Lu = u, u = Un(rc, "onSelect"), 0 < u.length && (t = new Ye(
      "onSelect",
      "select",
      null,
      t,
      a
    ), l.push({ event: t, listeners: u }), t.target = tu)));
  }
  function Na(l, t) {
    var a = {};
    return a[l.toLowerCase()] = t.toLowerCase(), a["Webkit" + l] = "webkit" + t, a["Moz" + l] = "moz" + t, a;
  }
  var au = {
    animationend: Na("Animation", "AnimationEnd"),
    animationiteration: Na("Animation", "AnimationIteration"),
    animationstart: Na("Animation", "AnimationStart"),
    transitionrun: Na("Transition", "TransitionRun"),
    transitionstart: Na("Transition", "TransitionStart"),
    transitioncancel: Na("Transition", "TransitionCancel"),
    transitionend: Na("Transition", "TransitionEnd")
  }, _c = {}, vs = {};
  Gt && (vs = document.createElement("div").style, "AnimationEvent" in window || (delete au.animationend.animation, delete au.animationiteration.animation, delete au.animationstart.animation), "TransitionEvent" in window || delete au.transitionend.transition);
  function ja(l) {
    if (_c[l]) return _c[l];
    if (!au[l]) return l;
    var t = au[l], a;
    for (a in t)
      if (t.hasOwnProperty(a) && a in vs)
        return _c[l] = t[a];
    return l;
  }
  var os = ja("animationend"), ds = ja("animationiteration"), ys = ja("animationstart"), x0 = ja("transitionrun"), q0 = ja("transitionstart"), B0 = ja("transitioncancel"), ms = ja("transitionend"), hs = /* @__PURE__ */ new Map(), zc = "abort auxClick beforeToggle cancel canPlay canPlayThrough click close contextMenu copy cut drag dragEnd dragEnter dragExit dragLeave dragOver dragStart drop durationChange emptied encrypted ended error gotPointerCapture input invalid keyDown keyPress keyUp load loadedData loadedMetadata loadStart lostPointerCapture mouseDown mouseMove mouseOut mouseOver mouseUp paste pause play playing pointerCancel pointerDown pointerMove pointerOut pointerOver pointerUp progress rateChange reset resize seeked seeking stalled submit suspend timeUpdate touchCancel touchEnd touchStart volumeChange scroll toggle touchMove waiting wheel".split(
    " "
  );
  zc.push("scrollEnd");
  function Ot(l, t) {
    hs.set(l, t), Da(t, [l]);
  }
  var Qe = typeof reportError == "function" ? reportError : function(l) {
    if (typeof window == "object" && typeof window.ErrorEvent == "function") {
      var t = new window.ErrorEvent("error", {
        bubbles: !0,
        cancelable: !0,
        message: typeof l == "object" && l !== null && typeof l.message == "string" ? String(l.message) : String(l),
        error: l
      });
      if (!window.dispatchEvent(t)) return;
    } else if (typeof process == "object" && typeof process.emit == "function") {
      process.emit("uncaughtException", l);
      return;
    }
    console.error(l);
  }, bt = [], uu = 0, Ec = 0;
  function Ze() {
    for (var l = uu, t = Ec = uu = 0; t < l; ) {
      var a = bt[t];
      bt[t++] = null;
      var u = bt[t];
      bt[t++] = null;
      var e = bt[t];
      bt[t++] = null;
      var n = bt[t];
      if (bt[t++] = null, u !== null && e !== null) {
        var c = u.pending;
        c === null ? e.next = e : (e.next = c.next, c.next = e), u.pending = e;
      }
      n !== 0 && gs(a, e, n);
    }
  }
  function Le(l, t, a, u) {
    bt[uu++] = l, bt[uu++] = t, bt[uu++] = a, bt[uu++] = u, Ec |= u, l.lanes |= u, l = l.alternate, l !== null && (l.lanes |= u);
  }
  function Tc(l, t, a, u) {
    return Le(l, t, a, u), Ve(l);
  }
  function Ha(l, t) {
    return Le(l, null, null, t), Ve(l);
  }
  function gs(l, t, a) {
    l.lanes |= a;
    var u = l.alternate;
    u !== null && (u.lanes |= a);
    for (var e = !1, n = l.return; n !== null; )
      n.childLanes |= a, u = n.alternate, u !== null && (u.childLanes |= a), n.tag === 22 && (l = n.stateNode, l === null || l._visibility & 1 || (e = !0)), l = n, n = n.return;
    return l.tag === 3 ? (n = l.stateNode, e && t !== null && (e = 31 - st(a), l = n.hiddenUpdates, u = l[e], u === null ? l[e] = [t] : u.push(t), t.lane = a | 536870912), n) : null;
  }
  function Ve(l) {
    if (50 < oe)
      throw oe = 0, Hi = null, Error(d(185));
    for (var t = l.return; t !== null; )
      l = t, t = l.return;
    return l.tag === 3 ? l.stateNode : null;
  }
  var eu = {};
  function C0(l, t, a, u) {
    this.tag = l, this.key = a, this.sibling = this.child = this.return = this.stateNode = this.type = this.elementType = null, this.index = 0, this.refCleanup = this.ref = null, this.pendingProps = t, this.dependencies = this.memoizedState = this.updateQueue = this.memoizedProps = null, this.mode = u, this.subtreeFlags = this.flags = 0, this.deletions = null, this.childLanes = this.lanes = 0, this.alternate = null;
  }
  function ot(l, t, a, u) {
    return new C0(l, t, a, u);
  }
  function Ac(l) {
    return l = l.prototype, !(!l || !l.isReactComponent);
  }
  function Xt(l, t) {
    var a = l.alternate;
    return a === null ? (a = ot(
      l.tag,
      t,
      l.key,
      l.mode
    ), a.elementType = l.elementType, a.type = l.type, a.stateNode = l.stateNode, a.alternate = l, l.alternate = a) : (a.pendingProps = t, a.type = l.type, a.flags = 0, a.subtreeFlags = 0, a.deletions = null), a.flags = l.flags & 65011712, a.childLanes = l.childLanes, a.lanes = l.lanes, a.child = l.child, a.memoizedProps = l.memoizedProps, a.memoizedState = l.memoizedState, a.updateQueue = l.updateQueue, t = l.dependencies, a.dependencies = t === null ? null : { lanes: t.lanes, firstContext: t.firstContext }, a.sibling = l.sibling, a.index = l.index, a.ref = l.ref, a.refCleanup = l.refCleanup, a;
  }
  function Ss(l, t) {
    l.flags &= 65011714;
    var a = l.alternate;
    return a === null ? (l.childLanes = 0, l.lanes = t, l.child = null, l.subtreeFlags = 0, l.memoizedProps = null, l.memoizedState = null, l.updateQueue = null, l.dependencies = null, l.stateNode = null) : (l.childLanes = a.childLanes, l.lanes = a.lanes, l.child = a.child, l.subtreeFlags = 0, l.deletions = null, l.memoizedProps = a.memoizedProps, l.memoizedState = a.memoizedState, l.updateQueue = a.updateQueue, l.type = a.type, t = a.dependencies, l.dependencies = t === null ? null : {
      lanes: t.lanes,
      firstContext: t.firstContext
    }), l;
  }
  function Ke(l, t, a, u, e, n) {
    var c = 0;
    if (u = l, typeof l == "function") Ac(l) && (c = 1);
    else if (typeof l == "string")
      c = Zy(
        l,
        a,
        H.current
      ) ? 26 : l === "html" || l === "head" || l === "body" ? 27 : 5;
    else
      l: switch (l) {
        case nt:
          return l = ot(31, a, t, e), l.elementType = nt, l.lanes = n, l;
        case Rl:
          return Ra(a.children, e, n, t);
        case El:
          c = 8, e |= 24;
          break;
        case rl:
          return l = ot(12, a, t, e | 2), l.elementType = rl, l.lanes = n, l;
        case et:
          return l = ot(13, a, t, e), l.elementType = et, l.lanes = n, l;
        case Gl:
          return l = ot(19, a, t, e), l.elementType = Gl, l.lanes = n, l;
        default:
          if (typeof l == "object" && l !== null)
            switch (l.$$typeof) {
              case Tl:
                c = 10;
                break l;
              case Yl:
                c = 9;
                break l;
              case kl:
                c = 11;
                break l;
              case V:
                c = 14;
                break l;
              case Xl:
                c = 16, u = null;
                break l;
            }
          c = 29, a = Error(
            d(130, l === null ? "null" : typeof l, "")
          ), u = null;
      }
    return t = ot(c, a, t, e), t.elementType = l, t.type = u, t.lanes = n, t;
  }
  function Ra(l, t, a, u) {
    return l = ot(7, l, u, t), l.lanes = a, l;
  }
  function pc(l, t, a) {
    return l = ot(6, l, null, t), l.lanes = a, l;
  }
  function rs(l) {
    var t = ot(18, null, null, 0);
    return t.stateNode = l, t;
  }
  function Mc(l, t, a) {
    return t = ot(
      4,
      l.children !== null ? l.children : [],
      l.key,
      t
    ), t.lanes = a, t.stateNode = {
      containerInfo: l.containerInfo,
      pendingChildren: null,
      implementation: l.implementation
    }, t;
  }
  var bs = /* @__PURE__ */ new WeakMap();
  function _t(l, t) {
    if (typeof l == "object" && l !== null) {
      var a = bs.get(l);
      return a !== void 0 ? a : (t = {
        value: l,
        source: t,
        stack: rf(t)
      }, bs.set(l, t), t);
    }
    return {
      value: l,
      source: t,
      stack: rf(t)
    };
  }
  var nu = [], cu = 0, Je = null, Vu = 0, zt = [], Et = 0, ea = null, jt = 1, Ht = "";
  function Qt(l, t) {
    nu[cu++] = Vu, nu[cu++] = Je, Je = l, Vu = t;
  }
  function _s(l, t, a) {
    zt[Et++] = jt, zt[Et++] = Ht, zt[Et++] = ea, ea = l;
    var u = jt;
    l = Ht;
    var e = 32 - st(u) - 1;
    u &= ~(1 << e), a += 1;
    var n = 32 - st(t) + e;
    if (30 < n) {
      var c = e - e % 5;
      n = (u & (1 << c) - 1).toString(32), u >>= c, e -= c, jt = 1 << 32 - st(t) + e | a << e | u, Ht = n + l;
    } else
      jt = 1 << n | a << e | u, Ht = l;
  }
  function Oc(l) {
    l.return !== null && (Qt(l, 1), _s(l, 1, 0));
  }
  function Dc(l) {
    for (; l === Je; )
      Je = nu[--cu], nu[cu] = null, Vu = nu[--cu], nu[cu] = null;
    for (; l === ea; )
      ea = zt[--Et], zt[Et] = null, Ht = zt[--Et], zt[Et] = null, jt = zt[--Et], zt[Et] = null;
  }
  function zs(l, t) {
    zt[Et++] = jt, zt[Et++] = Ht, zt[Et++] = ea, jt = t.id, Ht = t.overflow, ea = l;
  }
  var Ll = null, hl = null, $ = !1, na = null, Tt = !1, Uc = Error(d(519));
  function ca(l) {
    var t = Error(
      d(
        418,
        1 < arguments.length && arguments[1] !== void 0 && arguments[1] ? "text" : "HTML",
        ""
      )
    );
    throw Ku(_t(t, l)), Uc;
  }
  function Es(l) {
    var t = l.stateNode, a = l.type, u = l.memoizedProps;
    switch (t[Zl] = l, t[Fl] = u, a) {
      case "dialog":
        w("cancel", t), w("close", t);
        break;
      case "iframe":
      case "object":
      case "embed":
        w("load", t);
        break;
      case "video":
      case "audio":
        for (a = 0; a < ye.length; a++)
          w(ye[a], t);
        break;
      case "source":
        w("error", t);
        break;
      case "img":
      case "image":
      case "link":
        w("error", t), w("load", t);
        break;
      case "details":
        w("toggle", t);
        break;
      case "input":
        w("invalid", t), qf(
          t,
          u.value,
          u.defaultValue,
          u.checked,
          u.defaultChecked,
          u.type,
          u.name,
          !0
        );
        break;
      case "select":
        w("invalid", t);
        break;
      case "textarea":
        w("invalid", t), Cf(t, u.value, u.defaultValue, u.children);
    }
    a = u.children, typeof a != "string" && typeof a != "number" && typeof a != "bigint" || t.textContent === "" + a || u.suppressHydrationWarning === !0 || Xo(t.textContent, a) ? (u.popover != null && (w("beforetoggle", t), w("toggle", t)), u.onScroll != null && w("scroll", t), u.onScrollEnd != null && w("scrollend", t), u.onClick != null && (t.onclick = Yt), t = !0) : t = !1, t || ca(l, !0);
  }
  function Ts(l) {
    for (Ll = l.return; Ll; )
      switch (Ll.tag) {
        case 5:
        case 31:
        case 13:
          Tt = !1;
          return;
        case 27:
        case 3:
          Tt = !0;
          return;
        default:
          Ll = Ll.return;
      }
  }
  function iu(l) {
    if (l !== Ll) return !1;
    if (!$) return Ts(l), $ = !0, !1;
    var t = l.tag, a;
    if ((a = t !== 3 && t !== 27) && ((a = t === 5) && (a = l.type, a = !(a !== "form" && a !== "button") || wi(l.type, l.memoizedProps)), a = !a), a && hl && ca(l), Ts(l), t === 13) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(d(317));
      hl = Wo(l);
    } else if (t === 31) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(d(317));
      hl = Wo(l);
    } else
      t === 27 ? (t = hl, _a(l.type) ? (l = Ii, Ii = null, hl = l) : hl = t) : hl = Ll ? pt(l.stateNode.nextSibling) : null;
    return !0;
  }
  function xa() {
    hl = Ll = null, $ = !1;
  }
  function Nc() {
    var l = na;
    return l !== null && (at === null ? at = l : at.push.apply(
      at,
      l
    ), na = null), l;
  }
  function Ku(l) {
    na === null ? na = [l] : na.push(l);
  }
  var jc = v(null), qa = null, Zt = null;
  function ia(l, t, a) {
    O(jc, t._currentValue), t._currentValue = a;
  }
  function Lt(l) {
    l._currentValue = jc.current, E(jc);
  }
  function Hc(l, t, a) {
    for (; l !== null; ) {
      var u = l.alternate;
      if ((l.childLanes & t) !== t ? (l.childLanes |= t, u !== null && (u.childLanes |= t)) : u !== null && (u.childLanes & t) !== t && (u.childLanes |= t), l === a) break;
      l = l.return;
    }
  }
  function Rc(l, t, a, u) {
    var e = l.child;
    for (e !== null && (e.return = l); e !== null; ) {
      var n = e.dependencies;
      if (n !== null) {
        var c = e.child;
        n = n.firstContext;
        l: for (; n !== null; ) {
          var i = n;
          n = e;
          for (var f = 0; f < t.length; f++)
            if (i.context === t[f]) {
              n.lanes |= a, i = n.alternate, i !== null && (i.lanes |= a), Hc(
                n.return,
                a,
                l
              ), u || (c = null);
              break l;
            }
          n = i.next;
        }
      } else if (e.tag === 18) {
        if (c = e.return, c === null) throw Error(d(341));
        c.lanes |= a, n = c.alternate, n !== null && (n.lanes |= a), Hc(c, a, l), c = null;
      } else c = e.child;
      if (c !== null) c.return = e;
      else
        for (c = e; c !== null; ) {
          if (c === l) {
            c = null;
            break;
          }
          if (e = c.sibling, e !== null) {
            e.return = c.return, c = e;
            break;
          }
          c = c.return;
        }
      e = c;
    }
  }
  function fu(l, t, a, u) {
    l = null;
    for (var e = t, n = !1; e !== null; ) {
      if (!n) {
        if ((e.flags & 524288) !== 0) n = !0;
        else if ((e.flags & 262144) !== 0) break;
      }
      if (e.tag === 10) {
        var c = e.alternate;
        if (c === null) throw Error(d(387));
        if (c = c.memoizedProps, c !== null) {
          var i = e.type;
          vt(e.pendingProps.value, c.value) || (l !== null ? l.push(i) : l = [i]);
        }
      } else if (e === al.current) {
        if (c = e.alternate, c === null) throw Error(d(387));
        c.memoizedState.memoizedState !== e.memoizedState.memoizedState && (l !== null ? l.push(re) : l = [re]);
      }
      e = e.return;
    }
    l !== null && Rc(
      t,
      l,
      a,
      u
    ), t.flags |= 262144;
  }
  function we(l) {
    for (l = l.firstContext; l !== null; ) {
      if (!vt(
        l.context._currentValue,
        l.memoizedValue
      ))
        return !0;
      l = l.next;
    }
    return !1;
  }
  function Ba(l) {
    qa = l, Zt = null, l = l.dependencies, l !== null && (l.firstContext = null);
  }
  function Vl(l) {
    return As(qa, l);
  }
  function ke(l, t) {
    return qa === null && Ba(l), As(l, t);
  }
  function As(l, t) {
    var a = t._currentValue;
    if (t = { context: t, memoizedValue: a, next: null }, Zt === null) {
      if (l === null) throw Error(d(308));
      Zt = t, l.dependencies = { lanes: 0, firstContext: t }, l.flags |= 524288;
    } else Zt = Zt.next = t;
    return a;
  }
  var Y0 = typeof AbortController < "u" ? AbortController : function() {
    var l = [], t = this.signal = {
      aborted: !1,
      addEventListener: function(a, u) {
        l.push(u);
      }
    };
    this.abort = function() {
      t.aborted = !0, l.forEach(function(a) {
        return a();
      });
    };
  }, G0 = _.unstable_scheduleCallback, X0 = _.unstable_NormalPriority, Ol = {
    $$typeof: Tl,
    Consumer: null,
    Provider: null,
    _currentValue: null,
    _currentValue2: null,
    _threadCount: 0
  };
  function xc() {
    return {
      controller: new Y0(),
      data: /* @__PURE__ */ new Map(),
      refCount: 0
    };
  }
  function Ju(l) {
    l.refCount--, l.refCount === 0 && G0(X0, function() {
      l.controller.abort();
    });
  }
  var wu = null, qc = 0, su = 0, vu = null;
  function Q0(l, t) {
    if (wu === null) {
      var a = wu = [];
      qc = 0, su = Yi(), vu = {
        status: "pending",
        value: void 0,
        then: function(u) {
          a.push(u);
        }
      };
    }
    return qc++, t.then(ps, ps), t;
  }
  function ps() {
    if (--qc === 0 && wu !== null) {
      vu !== null && (vu.status = "fulfilled");
      var l = wu;
      wu = null, su = 0, vu = null;
      for (var t = 0; t < l.length; t++) (0, l[t])();
    }
  }
  function Z0(l, t) {
    var a = [], u = {
      status: "pending",
      value: null,
      reason: null,
      then: function(e) {
        a.push(e);
      }
    };
    return l.then(
      function() {
        u.status = "fulfilled", u.value = t;
        for (var e = 0; e < a.length; e++) (0, a[e])(t);
      },
      function(e) {
        for (u.status = "rejected", u.reason = e, e = 0; e < a.length; e++)
          (0, a[e])(void 0);
      }
    ), u;
  }
  var Ms = S.S;
  S.S = function(l, t) {
    so = it(), typeof t == "object" && t !== null && typeof t.then == "function" && Q0(l, t), Ms !== null && Ms(l, t);
  };
  var Ca = v(null);
  function Bc() {
    var l = Ca.current;
    return l !== null ? l : dl.pooledCache;
  }
  function We(l, t) {
    t === null ? O(Ca, Ca.current) : O(Ca, t.pool);
  }
  function Os() {
    var l = Bc();
    return l === null ? null : { parent: Ol._currentValue, pool: l };
  }
  var ou = Error(d(460)), Cc = Error(d(474)), $e = Error(d(542)), Fe = { then: function() {
  } };
  function Ds(l) {
    return l = l.status, l === "fulfilled" || l === "rejected";
  }
  function Us(l, t, a) {
    switch (a = l[a], a === void 0 ? l.push(t) : a !== t && (t.then(Yt, Yt), t = a), t.status) {
      case "fulfilled":
        return t.value;
      case "rejected":
        throw l = t.reason, js(l), l;
      default:
        if (typeof t.status == "string") t.then(Yt, Yt);
        else {
          if (l = dl, l !== null && 100 < l.shellSuspendCounter)
            throw Error(d(482));
          l = t, l.status = "pending", l.then(
            function(u) {
              if (t.status === "pending") {
                var e = t;
                e.status = "fulfilled", e.value = u;
              }
            },
            function(u) {
              if (t.status === "pending") {
                var e = t;
                e.status = "rejected", e.reason = u;
              }
            }
          );
        }
        switch (t.status) {
          case "fulfilled":
            return t.value;
          case "rejected":
            throw l = t.reason, js(l), l;
        }
        throw Ga = t, ou;
    }
  }
  function Ya(l) {
    try {
      var t = l._init;
      return t(l._payload);
    } catch (a) {
      throw a !== null && typeof a == "object" && typeof a.then == "function" ? (Ga = a, ou) : a;
    }
  }
  var Ga = null;
  function Ns() {
    if (Ga === null) throw Error(d(459));
    var l = Ga;
    return Ga = null, l;
  }
  function js(l) {
    if (l === ou || l === $e)
      throw Error(d(483));
  }
  var du = null, ku = 0;
  function Ie(l) {
    var t = ku;
    return ku += 1, du === null && (du = []), Us(du, l, t);
  }
  function Wu(l, t) {
    t = t.props.ref, l.ref = t !== void 0 ? t : null;
  }
  function Pe(l, t) {
    throw t.$$typeof === ll ? Error(d(525)) : (l = Object.prototype.toString.call(t), Error(
      d(
        31,
        l === "[object Object]" ? "object with keys {" + Object.keys(t).join(", ") + "}" : l
      )
    ));
  }
  function Hs(l) {
    function t(o, s) {
      if (l) {
        var y = o.deletions;
        y === null ? (o.deletions = [s], o.flags |= 16) : y.push(s);
      }
    }
    function a(o, s) {
      if (!l) return null;
      for (; s !== null; )
        t(o, s), s = s.sibling;
      return null;
    }
    function u(o) {
      for (var s = /* @__PURE__ */ new Map(); o !== null; )
        o.key !== null ? s.set(o.key, o) : s.set(o.index, o), o = o.sibling;
      return s;
    }
    function e(o, s) {
      return o = Xt(o, s), o.index = 0, o.sibling = null, o;
    }
    function n(o, s, y) {
      return o.index = y, l ? (y = o.alternate, y !== null ? (y = y.index, y < s ? (o.flags |= 67108866, s) : y) : (o.flags |= 67108866, s)) : (o.flags |= 1048576, s);
    }
    function c(o) {
      return l && o.alternate === null && (o.flags |= 67108866), o;
    }
    function i(o, s, y, b) {
      return s === null || s.tag !== 6 ? (s = pc(y, o.mode, b), s.return = o, s) : (s = e(s, y), s.return = o, s);
    }
    function f(o, s, y, b) {
      var q = y.type;
      return q === Rl ? r(
        o,
        s,
        y.props.children,
        b,
        y.key
      ) : s !== null && (s.elementType === q || typeof q == "object" && q !== null && q.$$typeof === Xl && Ya(q) === s.type) ? (s = e(s, y.props), Wu(s, y), s.return = o, s) : (s = Ke(
        y.type,
        y.key,
        y.props,
        null,
        o.mode,
        b
      ), Wu(s, y), s.return = o, s);
    }
    function m(o, s, y, b) {
      return s === null || s.tag !== 4 || s.stateNode.containerInfo !== y.containerInfo || s.stateNode.implementation !== y.implementation ? (s = Mc(y, o.mode, b), s.return = o, s) : (s = e(s, y.children || []), s.return = o, s);
    }
    function r(o, s, y, b, q) {
      return s === null || s.tag !== 7 ? (s = Ra(
        y,
        o.mode,
        b,
        q
      ), s.return = o, s) : (s = e(s, y), s.return = o, s);
    }
    function z(o, s, y) {
      if (typeof s == "string" && s !== "" || typeof s == "number" || typeof s == "bigint")
        return s = pc(
          "" + s,
          o.mode,
          y
        ), s.return = o, s;
      if (typeof s == "object" && s !== null) {
        switch (s.$$typeof) {
          case Hl:
            return y = Ke(
              s.type,
              s.key,
              s.props,
              null,
              o.mode,
              y
            ), Wu(y, s), y.return = o, y;
          case bl:
            return s = Mc(
              s,
              o.mode,
              y
            ), s.return = o, s;
          case Xl:
            return s = Ya(s), z(o, s, y);
        }
        if (tl(s) || Ql(s))
          return s = Ra(
            s,
            o.mode,
            y,
            null
          ), s.return = o, s;
        if (typeof s.then == "function")
          return z(o, Ie(s), y);
        if (s.$$typeof === Tl)
          return z(
            o,
            ke(o, s),
            y
          );
        Pe(o, s);
      }
      return null;
    }
    function h(o, s, y, b) {
      var q = s !== null ? s.key : null;
      if (typeof y == "string" && y !== "" || typeof y == "number" || typeof y == "bigint")
        return q !== null ? null : i(o, s, "" + y, b);
      if (typeof y == "object" && y !== null) {
        switch (y.$$typeof) {
          case Hl:
            return y.key === q ? f(o, s, y, b) : null;
          case bl:
            return y.key === q ? m(o, s, y, b) : null;
          case Xl:
            return y = Ya(y), h(o, s, y, b);
        }
        if (tl(y) || Ql(y))
          return q !== null ? null : r(o, s, y, b, null);
        if (typeof y.then == "function")
          return h(
            o,
            s,
            Ie(y),
            b
          );
        if (y.$$typeof === Tl)
          return h(
            o,
            s,
            ke(o, y),
            b
          );
        Pe(o, y);
      }
      return null;
    }
    function g(o, s, y, b, q) {
      if (typeof b == "string" && b !== "" || typeof b == "number" || typeof b == "bigint")
        return o = o.get(y) || null, i(s, o, "" + b, q);
      if (typeof b == "object" && b !== null) {
        switch (b.$$typeof) {
          case Hl:
            return o = o.get(
              b.key === null ? y : b.key
            ) || null, f(s, o, b, q);
          case bl:
            return o = o.get(
              b.key === null ? y : b.key
            ) || null, m(s, o, b, q);
          case Xl:
            return b = Ya(b), g(
              o,
              s,
              y,
              b,
              q
            );
        }
        if (tl(b) || Ql(b))
          return o = o.get(y) || null, r(s, o, b, q, null);
        if (typeof b.then == "function")
          return g(
            o,
            s,
            y,
            Ie(b),
            q
          );
        if (b.$$typeof === Tl)
          return g(
            o,
            s,
            y,
            ke(s, b),
            q
          );
        Pe(s, b);
      }
      return null;
    }
    function N(o, s, y, b) {
      for (var q = null, F = null, R = s, Q = s = 0, W = null; R !== null && Q < y.length; Q++) {
        R.index > Q ? (W = R, R = null) : W = R.sibling;
        var I = h(
          o,
          R,
          y[Q],
          b
        );
        if (I === null) {
          R === null && (R = W);
          break;
        }
        l && R && I.alternate === null && t(o, R), s = n(I, s, Q), F === null ? q = I : F.sibling = I, F = I, R = W;
      }
      if (Q === y.length)
        return a(o, R), $ && Qt(o, Q), q;
      if (R === null) {
        for (; Q < y.length; Q++)
          R = z(o, y[Q], b), R !== null && (s = n(
            R,
            s,
            Q
          ), F === null ? q = R : F.sibling = R, F = R);
        return $ && Qt(o, Q), q;
      }
      for (R = u(R); Q < y.length; Q++)
        W = g(
          R,
          o,
          Q,
          y[Q],
          b
        ), W !== null && (l && W.alternate !== null && R.delete(
          W.key === null ? Q : W.key
        ), s = n(
          W,
          s,
          Q
        ), F === null ? q = W : F.sibling = W, F = W);
      return l && R.forEach(function(pa) {
        return t(o, pa);
      }), $ && Qt(o, Q), q;
    }
    function B(o, s, y, b) {
      if (y == null) throw Error(d(151));
      for (var q = null, F = null, R = s, Q = s = 0, W = null, I = y.next(); R !== null && !I.done; Q++, I = y.next()) {
        R.index > Q ? (W = R, R = null) : W = R.sibling;
        var pa = h(o, R, I.value, b);
        if (pa === null) {
          R === null && (R = W);
          break;
        }
        l && R && pa.alternate === null && t(o, R), s = n(pa, s, Q), F === null ? q = pa : F.sibling = pa, F = pa, R = W;
      }
      if (I.done)
        return a(o, R), $ && Qt(o, Q), q;
      if (R === null) {
        for (; !I.done; Q++, I = y.next())
          I = z(o, I.value, b), I !== null && (s = n(I, s, Q), F === null ? q = I : F.sibling = I, F = I);
        return $ && Qt(o, Q), q;
      }
      for (R = u(R); !I.done; Q++, I = y.next())
        I = g(R, o, Q, I.value, b), I !== null && (l && I.alternate !== null && R.delete(I.key === null ? Q : I.key), s = n(I, s, Q), F === null ? q = I : F.sibling = I, F = I);
      return l && R.forEach(function(Py) {
        return t(o, Py);
      }), $ && Qt(o, Q), q;
    }
    function vl(o, s, y, b) {
      if (typeof y == "object" && y !== null && y.type === Rl && y.key === null && (y = y.props.children), typeof y == "object" && y !== null) {
        switch (y.$$typeof) {
          case Hl:
            l: {
              for (var q = y.key; s !== null; ) {
                if (s.key === q) {
                  if (q = y.type, q === Rl) {
                    if (s.tag === 7) {
                      a(
                        o,
                        s.sibling
                      ), b = e(
                        s,
                        y.props.children
                      ), b.return = o, o = b;
                      break l;
                    }
                  } else if (s.elementType === q || typeof q == "object" && q !== null && q.$$typeof === Xl && Ya(q) === s.type) {
                    a(
                      o,
                      s.sibling
                    ), b = e(s, y.props), Wu(b, y), b.return = o, o = b;
                    break l;
                  }
                  a(o, s);
                  break;
                } else t(o, s);
                s = s.sibling;
              }
              y.type === Rl ? (b = Ra(
                y.props.children,
                o.mode,
                b,
                y.key
              ), b.return = o, o = b) : (b = Ke(
                y.type,
                y.key,
                y.props,
                null,
                o.mode,
                b
              ), Wu(b, y), b.return = o, o = b);
            }
            return c(o);
          case bl:
            l: {
              for (q = y.key; s !== null; ) {
                if (s.key === q)
                  if (s.tag === 4 && s.stateNode.containerInfo === y.containerInfo && s.stateNode.implementation === y.implementation) {
                    a(
                      o,
                      s.sibling
                    ), b = e(s, y.children || []), b.return = o, o = b;
                    break l;
                  } else {
                    a(o, s);
                    break;
                  }
                else t(o, s);
                s = s.sibling;
              }
              b = Mc(y, o.mode, b), b.return = o, o = b;
            }
            return c(o);
          case Xl:
            return y = Ya(y), vl(
              o,
              s,
              y,
              b
            );
        }
        if (tl(y))
          return N(
            o,
            s,
            y,
            b
          );
        if (Ql(y)) {
          if (q = Ql(y), typeof q != "function") throw Error(d(150));
          return y = q.call(y), B(
            o,
            s,
            y,
            b
          );
        }
        if (typeof y.then == "function")
          return vl(
            o,
            s,
            Ie(y),
            b
          );
        if (y.$$typeof === Tl)
          return vl(
            o,
            s,
            ke(o, y),
            b
          );
        Pe(o, y);
      }
      return typeof y == "string" && y !== "" || typeof y == "number" || typeof y == "bigint" ? (y = "" + y, s !== null && s.tag === 6 ? (a(o, s.sibling), b = e(s, y), b.return = o, o = b) : (a(o, s), b = pc(y, o.mode, b), b.return = o, o = b), c(o)) : a(o, s);
    }
    return function(o, s, y, b) {
      try {
        ku = 0;
        var q = vl(
          o,
          s,
          y,
          b
        );
        return du = null, q;
      } catch (R) {
        if (R === ou || R === $e) throw R;
        var F = ot(29, R, null, o.mode);
        return F.lanes = b, F.return = o, F;
      } finally {
      }
    };
  }
  var Xa = Hs(!0), Rs = Hs(!1), fa = !1;
  function Yc(l) {
    l.updateQueue = {
      baseState: l.memoizedState,
      firstBaseUpdate: null,
      lastBaseUpdate: null,
      shared: { pending: null, lanes: 0, hiddenCallbacks: null },
      callbacks: null
    };
  }
  function Gc(l, t) {
    l = l.updateQueue, t.updateQueue === l && (t.updateQueue = {
      baseState: l.baseState,
      firstBaseUpdate: l.firstBaseUpdate,
      lastBaseUpdate: l.lastBaseUpdate,
      shared: l.shared,
      callbacks: null
    });
  }
  function sa(l) {
    return { lane: l, tag: 0, payload: null, callback: null, next: null };
  }
  function va(l, t, a) {
    var u = l.updateQueue;
    if (u === null) return null;
    if (u = u.shared, (P & 2) !== 0) {
      var e = u.pending;
      return e === null ? t.next = t : (t.next = e.next, e.next = t), u.pending = t, t = Ve(l), gs(l, null, a), t;
    }
    return Le(l, u, t, a), Ve(l);
  }
  function $u(l, t, a) {
    if (t = t.updateQueue, t !== null && (t = t.shared, (a & 4194048) !== 0)) {
      var u = t.lanes;
      u &= l.pendingLanes, a |= u, t.lanes = a, Af(l, a);
    }
  }
  function Xc(l, t) {
    var a = l.updateQueue, u = l.alternate;
    if (u !== null && (u = u.updateQueue, a === u)) {
      var e = null, n = null;
      if (a = a.firstBaseUpdate, a !== null) {
        do {
          var c = {
            lane: a.lane,
            tag: a.tag,
            payload: a.payload,
            callback: null,
            next: null
          };
          n === null ? e = n = c : n = n.next = c, a = a.next;
        } while (a !== null);
        n === null ? e = n = t : n = n.next = t;
      } else e = n = t;
      a = {
        baseState: u.baseState,
        firstBaseUpdate: e,
        lastBaseUpdate: n,
        shared: u.shared,
        callbacks: u.callbacks
      }, l.updateQueue = a;
      return;
    }
    l = a.lastBaseUpdate, l === null ? a.firstBaseUpdate = t : l.next = t, a.lastBaseUpdate = t;
  }
  var Qc = !1;
  function Fu() {
    if (Qc) {
      var l = vu;
      if (l !== null) throw l;
    }
  }
  function Iu(l, t, a, u) {
    Qc = !1;
    var e = l.updateQueue;
    fa = !1;
    var n = e.firstBaseUpdate, c = e.lastBaseUpdate, i = e.shared.pending;
    if (i !== null) {
      e.shared.pending = null;
      var f = i, m = f.next;
      f.next = null, c === null ? n = m : c.next = m, c = f;
      var r = l.alternate;
      r !== null && (r = r.updateQueue, i = r.lastBaseUpdate, i !== c && (i === null ? r.firstBaseUpdate = m : i.next = m, r.lastBaseUpdate = f));
    }
    if (n !== null) {
      var z = e.baseState;
      c = 0, r = m = f = null, i = n;
      do {
        var h = i.lane & -536870913, g = h !== i.lane;
        if (g ? (k & h) === h : (u & h) === h) {
          h !== 0 && h === su && (Qc = !0), r !== null && (r = r.next = {
            lane: 0,
            tag: i.tag,
            payload: i.payload,
            callback: null,
            next: null
          });
          l: {
            var N = l, B = i;
            h = t;
            var vl = a;
            switch (B.tag) {
              case 1:
                if (N = B.payload, typeof N == "function") {
                  z = N.call(vl, z, h);
                  break l;
                }
                z = N;
                break l;
              case 3:
                N.flags = N.flags & -65537 | 128;
              case 0:
                if (N = B.payload, h = typeof N == "function" ? N.call(vl, z, h) : N, h == null) break l;
                z = x({}, z, h);
                break l;
              case 2:
                fa = !0;
            }
          }
          h = i.callback, h !== null && (l.flags |= 64, g && (l.flags |= 8192), g = e.callbacks, g === null ? e.callbacks = [h] : g.push(h));
        } else
          g = {
            lane: h,
            tag: i.tag,
            payload: i.payload,
            callback: i.callback,
            next: null
          }, r === null ? (m = r = g, f = z) : r = r.next = g, c |= h;
        if (i = i.next, i === null) {
          if (i = e.shared.pending, i === null)
            break;
          g = i, i = g.next, g.next = null, e.lastBaseUpdate = g, e.shared.pending = null;
        }
      } while (!0);
      r === null && (f = z), e.baseState = f, e.firstBaseUpdate = m, e.lastBaseUpdate = r, n === null && (e.shared.lanes = 0), ha |= c, l.lanes = c, l.memoizedState = z;
    }
  }
  function xs(l, t) {
    if (typeof l != "function")
      throw Error(d(191, l));
    l.call(t);
  }
  function qs(l, t) {
    var a = l.callbacks;
    if (a !== null)
      for (l.callbacks = null, l = 0; l < a.length; l++)
        xs(a[l], t);
  }
  var yu = v(null), ln = v(0);
  function Bs(l, t) {
    l = It, O(ln, l), O(yu, t), It = l | t.baseLanes;
  }
  function Zc() {
    O(ln, It), O(yu, yu.current);
  }
  function Lc() {
    It = ln.current, E(yu), E(ln);
  }
  var dt = v(null), At = null;
  function oa(l) {
    var t = l.alternate;
    O(pl, pl.current & 1), O(dt, l), At === null && (t === null || yu.current !== null || t.memoizedState !== null) && (At = l);
  }
  function Vc(l) {
    O(pl, pl.current), O(dt, l), At === null && (At = l);
  }
  function Cs(l) {
    l.tag === 22 ? (O(pl, pl.current), O(dt, l), At === null && (At = l)) : da();
  }
  function da() {
    O(pl, pl.current), O(dt, dt.current);
  }
  function yt(l) {
    E(dt), At === l && (At = null), E(pl);
  }
  var pl = v(0);
  function tn(l) {
    for (var t = l; t !== null; ) {
      if (t.tag === 13) {
        var a = t.memoizedState;
        if (a !== null && (a = a.dehydrated, a === null || $i(a) || Fi(a)))
          return t;
      } else if (t.tag === 19 && (t.memoizedProps.revealOrder === "forwards" || t.memoizedProps.revealOrder === "backwards" || t.memoizedProps.revealOrder === "unstable_legacy-backwards" || t.memoizedProps.revealOrder === "together")) {
        if ((t.flags & 128) !== 0) return t;
      } else if (t.child !== null) {
        t.child.return = t, t = t.child;
        continue;
      }
      if (t === l) break;
      for (; t.sibling === null; ) {
        if (t.return === null || t.return === l) return null;
        t = t.return;
      }
      t.sibling.return = t.return, t = t.sibling;
    }
    return null;
  }
  var Vt = 0, X = null, fl = null, Dl = null, an = !1, mu = !1, Qa = !1, un = 0, Pu = 0, hu = null, L0 = 0;
  function _l() {
    throw Error(d(321));
  }
  function Kc(l, t) {
    if (t === null) return !1;
    for (var a = 0; a < t.length && a < l.length; a++)
      if (!vt(l[a], t[a])) return !1;
    return !0;
  }
  function Jc(l, t, a, u, e, n) {
    return Vt = n, X = t, t.memoizedState = null, t.updateQueue = null, t.lanes = 0, S.H = l === null || l.memoizedState === null ? _v : ii, Qa = !1, n = a(u, e), Qa = !1, mu && (n = Gs(
      t,
      a,
      u,
      e
    )), Ys(l), n;
  }
  function Ys(l) {
    S.H = ae;
    var t = fl !== null && fl.next !== null;
    if (Vt = 0, Dl = fl = X = null, an = !1, Pu = 0, hu = null, t) throw Error(d(300));
    l === null || Ul || (l = l.dependencies, l !== null && we(l) && (Ul = !0));
  }
  function Gs(l, t, a, u) {
    X = l;
    var e = 0;
    do {
      if (mu && (hu = null), Pu = 0, mu = !1, 25 <= e) throw Error(d(301));
      if (e += 1, Dl = fl = null, l.updateQueue != null) {
        var n = l.updateQueue;
        n.lastEffect = null, n.events = null, n.stores = null, n.memoCache != null && (n.memoCache.index = 0);
      }
      S.H = zv, n = t(a, u);
    } while (mu);
    return n;
  }
  function V0() {
    var l = S.H, t = l.useState()[0];
    return t = typeof t.then == "function" ? le(t) : t, l = l.useState()[0], (fl !== null ? fl.memoizedState : null) !== l && (X.flags |= 1024), t;
  }
  function wc() {
    var l = un !== 0;
    return un = 0, l;
  }
  function kc(l, t, a) {
    t.updateQueue = l.updateQueue, t.flags &= -2053, l.lanes &= ~a;
  }
  function Wc(l) {
    if (an) {
      for (l = l.memoizedState; l !== null; ) {
        var t = l.queue;
        t !== null && (t.pending = null), l = l.next;
      }
      an = !1;
    }
    Vt = 0, Dl = fl = X = null, mu = !1, Pu = un = 0, hu = null;
  }
  function $l() {
    var l = {
      memoizedState: null,
      baseState: null,
      baseQueue: null,
      queue: null,
      next: null
    };
    return Dl === null ? X.memoizedState = Dl = l : Dl = Dl.next = l, Dl;
  }
  function Ml() {
    if (fl === null) {
      var l = X.alternate;
      l = l !== null ? l.memoizedState : null;
    } else l = fl.next;
    var t = Dl === null ? X.memoizedState : Dl.next;
    if (t !== null)
      Dl = t, fl = l;
    else {
      if (l === null)
        throw X.alternate === null ? Error(d(467)) : Error(d(310));
      fl = l, l = {
        memoizedState: fl.memoizedState,
        baseState: fl.baseState,
        baseQueue: fl.baseQueue,
        queue: fl.queue,
        next: null
      }, Dl === null ? X.memoizedState = Dl = l : Dl = Dl.next = l;
    }
    return Dl;
  }
  function en() {
    return { lastEffect: null, events: null, stores: null, memoCache: null };
  }
  function le(l) {
    var t = Pu;
    return Pu += 1, hu === null && (hu = []), l = Us(hu, l, t), t = X, (Dl === null ? t.memoizedState : Dl.next) === null && (t = t.alternate, S.H = t === null || t.memoizedState === null ? _v : ii), l;
  }
  function nn(l) {
    if (l !== null && typeof l == "object") {
      if (typeof l.then == "function") return le(l);
      if (l.$$typeof === Tl) return Vl(l);
    }
    throw Error(d(438, String(l)));
  }
  function $c(l) {
    var t = null, a = X.updateQueue;
    if (a !== null && (t = a.memoCache), t == null) {
      var u = X.alternate;
      u !== null && (u = u.updateQueue, u !== null && (u = u.memoCache, u != null && (t = {
        data: u.data.map(function(e) {
          return e.slice();
        }),
        index: 0
      })));
    }
    if (t == null && (t = { data: [], index: 0 }), a === null && (a = en(), X.updateQueue = a), a.memoCache = t, a = t.data[t.index], a === void 0)
      for (a = t.data[t.index] = Array(l), u = 0; u < l; u++)
        a[u] = Bt;
    return t.index++, a;
  }
  function Kt(l, t) {
    return typeof t == "function" ? t(l) : t;
  }
  function cn(l) {
    var t = Ml();
    return Fc(t, fl, l);
  }
  function Fc(l, t, a) {
    var u = l.queue;
    if (u === null) throw Error(d(311));
    u.lastRenderedReducer = a;
    var e = l.baseQueue, n = u.pending;
    if (n !== null) {
      if (e !== null) {
        var c = e.next;
        e.next = n.next, n.next = c;
      }
      t.baseQueue = e = n, u.pending = null;
    }
    if (n = l.baseState, e === null) l.memoizedState = n;
    else {
      t = e.next;
      var i = c = null, f = null, m = t, r = !1;
      do {
        var z = m.lane & -536870913;
        if (z !== m.lane ? (k & z) === z : (Vt & z) === z) {
          var h = m.revertLane;
          if (h === 0)
            f !== null && (f = f.next = {
              lane: 0,
              revertLane: 0,
              gesture: null,
              action: m.action,
              hasEagerState: m.hasEagerState,
              eagerState: m.eagerState,
              next: null
            }), z === su && (r = !0);
          else if ((Vt & h) === h) {
            m = m.next, h === su && (r = !0);
            continue;
          } else
            z = {
              lane: 0,
              revertLane: m.revertLane,
              gesture: null,
              action: m.action,
              hasEagerState: m.hasEagerState,
              eagerState: m.eagerState,
              next: null
            }, f === null ? (i = f = z, c = n) : f = f.next = z, X.lanes |= h, ha |= h;
          z = m.action, Qa && a(n, z), n = m.hasEagerState ? m.eagerState : a(n, z);
        } else
          h = {
            lane: z,
            revertLane: m.revertLane,
            gesture: m.gesture,
            action: m.action,
            hasEagerState: m.hasEagerState,
            eagerState: m.eagerState,
            next: null
          }, f === null ? (i = f = h, c = n) : f = f.next = h, X.lanes |= z, ha |= z;
        m = m.next;
      } while (m !== null && m !== t);
      if (f === null ? c = n : f.next = i, !vt(n, l.memoizedState) && (Ul = !0, r && (a = vu, a !== null)))
        throw a;
      l.memoizedState = n, l.baseState = c, l.baseQueue = f, u.lastRenderedState = n;
    }
    return e === null && (u.lanes = 0), [l.memoizedState, u.dispatch];
  }
  function Ic(l) {
    var t = Ml(), a = t.queue;
    if (a === null) throw Error(d(311));
    a.lastRenderedReducer = l;
    var u = a.dispatch, e = a.pending, n = t.memoizedState;
    if (e !== null) {
      a.pending = null;
      var c = e = e.next;
      do
        n = l(n, c.action), c = c.next;
      while (c !== e);
      vt(n, t.memoizedState) || (Ul = !0), t.memoizedState = n, t.baseQueue === null && (t.baseState = n), a.lastRenderedState = n;
    }
    return [n, u];
  }
  function Xs(l, t, a) {
    var u = X, e = Ml(), n = $;
    if (n) {
      if (a === void 0) throw Error(d(407));
      a = a();
    } else a = t();
    var c = !vt(
      (fl || e).memoizedState,
      a
    );
    if (c && (e.memoizedState = a, Ul = !0), e = e.queue, ti(Ls.bind(null, u, e, l), [
      l
    ]), e.getSnapshot !== t || c || Dl !== null && Dl.memoizedState.tag & 1) {
      if (u.flags |= 2048, gu(
        9,
        { destroy: void 0 },
        Zs.bind(
          null,
          u,
          e,
          a,
          t
        ),
        null
      ), dl === null) throw Error(d(349));
      n || (Vt & 127) !== 0 || Qs(u, t, a);
    }
    return a;
  }
  function Qs(l, t, a) {
    l.flags |= 16384, l = { getSnapshot: t, value: a }, t = X.updateQueue, t === null ? (t = en(), X.updateQueue = t, t.stores = [l]) : (a = t.stores, a === null ? t.stores = [l] : a.push(l));
  }
  function Zs(l, t, a, u) {
    t.value = a, t.getSnapshot = u, Vs(t) && Ks(l);
  }
  function Ls(l, t, a) {
    return a(function() {
      Vs(t) && Ks(l);
    });
  }
  function Vs(l) {
    var t = l.getSnapshot;
    l = l.value;
    try {
      var a = t();
      return !vt(l, a);
    } catch {
      return !0;
    }
  }
  function Ks(l) {
    var t = Ha(l, 2);
    t !== null && ut(t, l, 2);
  }
  function Pc(l) {
    var t = $l();
    if (typeof l == "function") {
      var a = l;
      if (l = a(), Qa) {
        ta(!0);
        try {
          a();
        } finally {
          ta(!1);
        }
      }
    }
    return t.memoizedState = t.baseState = l, t.queue = {
      pending: null,
      lanes: 0,
      dispatch: null,
      lastRenderedReducer: Kt,
      lastRenderedState: l
    }, t;
  }
  function Js(l, t, a, u) {
    return l.baseState = a, Fc(
      l,
      fl,
      typeof u == "function" ? u : Kt
    );
  }
  function K0(l, t, a, u, e) {
    if (vn(l)) throw Error(d(485));
    if (l = t.action, l !== null) {
      var n = {
        payload: e,
        action: l,
        next: null,
        isTransition: !0,
        status: "pending",
        value: null,
        reason: null,
        listeners: [],
        then: function(c) {
          n.listeners.push(c);
        }
      };
      S.T !== null ? a(!0) : n.isTransition = !1, u(n), a = t.pending, a === null ? (n.next = t.pending = n, ws(t, n)) : (n.next = a.next, t.pending = a.next = n);
    }
  }
  function ws(l, t) {
    var a = t.action, u = t.payload, e = l.state;
    if (t.isTransition) {
      var n = S.T, c = {};
      S.T = c;
      try {
        var i = a(e, u), f = S.S;
        f !== null && f(c, i), ks(l, t, i);
      } catch (m) {
        li(l, t, m);
      } finally {
        n !== null && c.types !== null && (n.types = c.types), S.T = n;
      }
    } else
      try {
        n = a(e, u), ks(l, t, n);
      } catch (m) {
        li(l, t, m);
      }
  }
  function ks(l, t, a) {
    a !== null && typeof a == "object" && typeof a.then == "function" ? a.then(
      function(u) {
        Ws(l, t, u);
      },
      function(u) {
        return li(l, t, u);
      }
    ) : Ws(l, t, a);
  }
  function Ws(l, t, a) {
    t.status = "fulfilled", t.value = a, $s(t), l.state = a, t = l.pending, t !== null && (a = t.next, a === t ? l.pending = null : (a = a.next, t.next = a, ws(l, a)));
  }
  function li(l, t, a) {
    var u = l.pending;
    if (l.pending = null, u !== null) {
      u = u.next;
      do
        t.status = "rejected", t.reason = a, $s(t), t = t.next;
      while (t !== u);
    }
    l.action = null;
  }
  function $s(l) {
    l = l.listeners;
    for (var t = 0; t < l.length; t++) (0, l[t])();
  }
  function Fs(l, t) {
    return t;
  }
  function Is(l, t) {
    if ($) {
      var a = dl.formState;
      if (a !== null) {
        l: {
          var u = X;
          if ($) {
            if (hl) {
              t: {
                for (var e = hl, n = Tt; e.nodeType !== 8; ) {
                  if (!n) {
                    e = null;
                    break t;
                  }
                  if (e = pt(
                    e.nextSibling
                  ), e === null) {
                    e = null;
                    break t;
                  }
                }
                n = e.data, e = n === "F!" || n === "F" ? e : null;
              }
              if (e) {
                hl = pt(
                  e.nextSibling
                ), u = e.data === "F!";
                break l;
              }
            }
            ca(u);
          }
          u = !1;
        }
        u && (t = a[0]);
      }
    }
    return a = $l(), a.memoizedState = a.baseState = t, u = {
      pending: null,
      lanes: 0,
      dispatch: null,
      lastRenderedReducer: Fs,
      lastRenderedState: t
    }, a.queue = u, a = Sv.bind(
      null,
      X,
      u
    ), u.dispatch = a, u = Pc(!1), n = ci.bind(
      null,
      X,
      !1,
      u.queue
    ), u = $l(), e = {
      state: t,
      dispatch: null,
      action: l,
      pending: null
    }, u.queue = e, a = K0.bind(
      null,
      X,
      e,
      n,
      a
    ), e.dispatch = a, u.memoizedState = l, [t, a, !1];
  }
  function Ps(l) {
    var t = Ml();
    return lv(t, fl, l);
  }
  function lv(l, t, a) {
    if (t = Fc(
      l,
      t,
      Fs
    )[0], l = cn(Kt)[0], typeof t == "object" && t !== null && typeof t.then == "function")
      try {
        var u = le(t);
      } catch (c) {
        throw c === ou ? $e : c;
      }
    else u = t;
    t = Ml();
    var e = t.queue, n = e.dispatch;
    return a !== t.memoizedState && (X.flags |= 2048, gu(
      9,
      { destroy: void 0 },
      J0.bind(null, e, a),
      null
    )), [u, n, l];
  }
  function J0(l, t) {
    l.action = t;
  }
  function tv(l) {
    var t = Ml(), a = fl;
    if (a !== null)
      return lv(t, a, l);
    Ml(), t = t.memoizedState, a = Ml();
    var u = a.queue.dispatch;
    return a.memoizedState = l, [t, u, !1];
  }
  function gu(l, t, a, u) {
    return l = { tag: l, create: a, deps: u, inst: t, next: null }, t = X.updateQueue, t === null && (t = en(), X.updateQueue = t), a = t.lastEffect, a === null ? t.lastEffect = l.next = l : (u = a.next, a.next = l, l.next = u, t.lastEffect = l), l;
  }
  function av() {
    return Ml().memoizedState;
  }
  function fn(l, t, a, u) {
    var e = $l();
    X.flags |= l, e.memoizedState = gu(
      1 | t,
      { destroy: void 0 },
      a,
      u === void 0 ? null : u
    );
  }
  function sn(l, t, a, u) {
    var e = Ml();
    u = u === void 0 ? null : u;
    var n = e.memoizedState.inst;
    fl !== null && u !== null && Kc(u, fl.memoizedState.deps) ? e.memoizedState = gu(t, n, a, u) : (X.flags |= l, e.memoizedState = gu(
      1 | t,
      n,
      a,
      u
    ));
  }
  function uv(l, t) {
    fn(8390656, 8, l, t);
  }
  function ti(l, t) {
    sn(2048, 8, l, t);
  }
  function w0(l) {
    X.flags |= 4;
    var t = X.updateQueue;
    if (t === null)
      t = en(), X.updateQueue = t, t.events = [l];
    else {
      var a = t.events;
      a === null ? t.events = [l] : a.push(l);
    }
  }
  function ev(l) {
    var t = Ml().memoizedState;
    return w0({ ref: t, nextImpl: l }), function() {
      if ((P & 2) !== 0) throw Error(d(440));
      return t.impl.apply(void 0, arguments);
    };
  }
  function nv(l, t) {
    return sn(4, 2, l, t);
  }
  function cv(l, t) {
    return sn(4, 4, l, t);
  }
  function iv(l, t) {
    if (typeof t == "function") {
      l = l();
      var a = t(l);
      return function() {
        typeof a == "function" ? a() : t(null);
      };
    }
    if (t != null)
      return l = l(), t.current = l, function() {
        t.current = null;
      };
  }
  function fv(l, t, a) {
    a = a != null ? a.concat([l]) : null, sn(4, 4, iv.bind(null, t, l), a);
  }
  function ai() {
  }
  function sv(l, t) {
    var a = Ml();
    t = t === void 0 ? null : t;
    var u = a.memoizedState;
    return t !== null && Kc(t, u[1]) ? u[0] : (a.memoizedState = [l, t], l);
  }
  function vv(l, t) {
    var a = Ml();
    t = t === void 0 ? null : t;
    var u = a.memoizedState;
    if (t !== null && Kc(t, u[1]))
      return u[0];
    if (u = l(), Qa) {
      ta(!0);
      try {
        l();
      } finally {
        ta(!1);
      }
    }
    return a.memoizedState = [u, t], u;
  }
  function ui(l, t, a) {
    return a === void 0 || (Vt & 1073741824) !== 0 && (k & 261930) === 0 ? l.memoizedState = t : (l.memoizedState = a, l = oo(), X.lanes |= l, ha |= l, a);
  }
  function ov(l, t, a, u) {
    return vt(a, t) ? a : yu.current !== null ? (l = ui(l, a, u), vt(l, t) || (Ul = !0), l) : (Vt & 42) === 0 || (Vt & 1073741824) !== 0 && (k & 261930) === 0 ? (Ul = !0, l.memoizedState = a) : (l = oo(), X.lanes |= l, ha |= l, t);
  }
  function dv(l, t, a, u, e) {
    var n = M.p;
    M.p = n !== 0 && 8 > n ? n : 8;
    var c = S.T, i = {};
    S.T = i, ci(l, !1, t, a);
    try {
      var f = e(), m = S.S;
      if (m !== null && m(i, f), f !== null && typeof f == "object" && typeof f.then == "function") {
        var r = Z0(
          f,
          u
        );
        te(
          l,
          t,
          r,
          gt(l)
        );
      } else
        te(
          l,
          t,
          u,
          gt(l)
        );
    } catch (z) {
      te(
        l,
        t,
        { then: function() {
        }, status: "rejected", reason: z },
        gt()
      );
    } finally {
      M.p = n, c !== null && i.types !== null && (c.types = i.types), S.T = c;
    }
  }
  function k0() {
  }
  function ei(l, t, a, u) {
    if (l.tag !== 5) throw Error(d(476));
    var e = yv(l).queue;
    dv(
      l,
      e,
      t,
      C,
      a === null ? k0 : function() {
        return mv(l), a(u);
      }
    );
  }
  function yv(l) {
    var t = l.memoizedState;
    if (t !== null) return t;
    t = {
      memoizedState: C,
      baseState: C,
      baseQueue: null,
      queue: {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: Kt,
        lastRenderedState: C
      },
      next: null
    };
    var a = {};
    return t.next = {
      memoizedState: a,
      baseState: a,
      baseQueue: null,
      queue: {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: Kt,
        lastRenderedState: a
      },
      next: null
    }, l.memoizedState = t, l = l.alternate, l !== null && (l.memoizedState = t), t;
  }
  function mv(l) {
    var t = yv(l);
    t.next === null && (t = l.alternate.memoizedState), te(
      l,
      t.next.queue,
      {},
      gt()
    );
  }
  function ni() {
    return Vl(re);
  }
  function hv() {
    return Ml().memoizedState;
  }
  function gv() {
    return Ml().memoizedState;
  }
  function W0(l) {
    for (var t = l.return; t !== null; ) {
      switch (t.tag) {
        case 24:
        case 3:
          var a = gt();
          l = sa(a);
          var u = va(t, l, a);
          u !== null && (ut(u, t, a), $u(u, t, a)), t = { cache: xc() }, l.payload = t;
          return;
      }
      t = t.return;
    }
  }
  function $0(l, t, a) {
    var u = gt();
    a = {
      lane: u,
      revertLane: 0,
      gesture: null,
      action: a,
      hasEagerState: !1,
      eagerState: null,
      next: null
    }, vn(l) ? rv(t, a) : (a = Tc(l, t, a, u), a !== null && (ut(a, l, u), bv(a, t, u)));
  }
  function Sv(l, t, a) {
    var u = gt();
    te(l, t, a, u);
  }
  function te(l, t, a, u) {
    var e = {
      lane: u,
      revertLane: 0,
      gesture: null,
      action: a,
      hasEagerState: !1,
      eagerState: null,
      next: null
    };
    if (vn(l)) rv(t, e);
    else {
      var n = l.alternate;
      if (l.lanes === 0 && (n === null || n.lanes === 0) && (n = t.lastRenderedReducer, n !== null))
        try {
          var c = t.lastRenderedState, i = n(c, a);
          if (e.hasEagerState = !0, e.eagerState = i, vt(i, c))
            return Le(l, t, e, 0), dl === null && Ze(), !1;
        } catch {
        } finally {
        }
      if (a = Tc(l, t, e, u), a !== null)
        return ut(a, l, u), bv(a, t, u), !0;
    }
    return !1;
  }
  function ci(l, t, a, u) {
    if (u = {
      lane: 2,
      revertLane: Yi(),
      gesture: null,
      action: u,
      hasEagerState: !1,
      eagerState: null,
      next: null
    }, vn(l)) {
      if (t) throw Error(d(479));
    } else
      t = Tc(
        l,
        a,
        u,
        2
      ), t !== null && ut(t, l, 2);
  }
  function vn(l) {
    var t = l.alternate;
    return l === X || t !== null && t === X;
  }
  function rv(l, t) {
    mu = an = !0;
    var a = l.pending;
    a === null ? t.next = t : (t.next = a.next, a.next = t), l.pending = t;
  }
  function bv(l, t, a) {
    if ((a & 4194048) !== 0) {
      var u = t.lanes;
      u &= l.pendingLanes, a |= u, t.lanes = a, Af(l, a);
    }
  }
  var ae = {
    readContext: Vl,
    use: nn,
    useCallback: _l,
    useContext: _l,
    useEffect: _l,
    useImperativeHandle: _l,
    useLayoutEffect: _l,
    useInsertionEffect: _l,
    useMemo: _l,
    useReducer: _l,
    useRef: _l,
    useState: _l,
    useDebugValue: _l,
    useDeferredValue: _l,
    useTransition: _l,
    useSyncExternalStore: _l,
    useId: _l,
    useHostTransitionStatus: _l,
    useFormState: _l,
    useActionState: _l,
    useOptimistic: _l,
    useMemoCache: _l,
    useCacheRefresh: _l
  };
  ae.useEffectEvent = _l;
  var _v = {
    readContext: Vl,
    use: nn,
    useCallback: function(l, t) {
      return $l().memoizedState = [
        l,
        t === void 0 ? null : t
      ], l;
    },
    useContext: Vl,
    useEffect: uv,
    useImperativeHandle: function(l, t, a) {
      a = a != null ? a.concat([l]) : null, fn(
        4194308,
        4,
        iv.bind(null, t, l),
        a
      );
    },
    useLayoutEffect: function(l, t) {
      return fn(4194308, 4, l, t);
    },
    useInsertionEffect: function(l, t) {
      fn(4, 2, l, t);
    },
    useMemo: function(l, t) {
      var a = $l();
      t = t === void 0 ? null : t;
      var u = l();
      if (Qa) {
        ta(!0);
        try {
          l();
        } finally {
          ta(!1);
        }
      }
      return a.memoizedState = [u, t], u;
    },
    useReducer: function(l, t, a) {
      var u = $l();
      if (a !== void 0) {
        var e = a(t);
        if (Qa) {
          ta(!0);
          try {
            a(t);
          } finally {
            ta(!1);
          }
        }
      } else e = t;
      return u.memoizedState = u.baseState = e, l = {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: l,
        lastRenderedState: e
      }, u.queue = l, l = l.dispatch = $0.bind(
        null,
        X,
        l
      ), [u.memoizedState, l];
    },
    useRef: function(l) {
      var t = $l();
      return l = { current: l }, t.memoizedState = l;
    },
    useState: function(l) {
      l = Pc(l);
      var t = l.queue, a = Sv.bind(null, X, t);
      return t.dispatch = a, [l.memoizedState, a];
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = $l();
      return ui(a, l, t);
    },
    useTransition: function() {
      var l = Pc(!1);
      return l = dv.bind(
        null,
        X,
        l.queue,
        !0,
        !1
      ), $l().memoizedState = l, [!1, l];
    },
    useSyncExternalStore: function(l, t, a) {
      var u = X, e = $l();
      if ($) {
        if (a === void 0)
          throw Error(d(407));
        a = a();
      } else {
        if (a = t(), dl === null)
          throw Error(d(349));
        (k & 127) !== 0 || Qs(u, t, a);
      }
      e.memoizedState = a;
      var n = { value: a, getSnapshot: t };
      return e.queue = n, uv(Ls.bind(null, u, n, l), [
        l
      ]), u.flags |= 2048, gu(
        9,
        { destroy: void 0 },
        Zs.bind(
          null,
          u,
          n,
          a,
          t
        ),
        null
      ), a;
    },
    useId: function() {
      var l = $l(), t = dl.identifierPrefix;
      if ($) {
        var a = Ht, u = jt;
        a = (u & ~(1 << 32 - st(u) - 1)).toString(32) + a, t = "_" + t + "R_" + a, a = un++, 0 < a && (t += "H" + a.toString(32)), t += "_";
      } else
        a = L0++, t = "_" + t + "r_" + a.toString(32) + "_";
      return l.memoizedState = t;
    },
    useHostTransitionStatus: ni,
    useFormState: Is,
    useActionState: Is,
    useOptimistic: function(l) {
      var t = $l();
      t.memoizedState = t.baseState = l;
      var a = {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: null,
        lastRenderedState: null
      };
      return t.queue = a, t = ci.bind(
        null,
        X,
        !0,
        a
      ), a.dispatch = t, [l, t];
    },
    useMemoCache: $c,
    useCacheRefresh: function() {
      return $l().memoizedState = W0.bind(
        null,
        X
      );
    },
    useEffectEvent: function(l) {
      var t = $l(), a = { impl: l };
      return t.memoizedState = a, function() {
        if ((P & 2) !== 0)
          throw Error(d(440));
        return a.impl.apply(void 0, arguments);
      };
    }
  }, ii = {
    readContext: Vl,
    use: nn,
    useCallback: sv,
    useContext: Vl,
    useEffect: ti,
    useImperativeHandle: fv,
    useInsertionEffect: nv,
    useLayoutEffect: cv,
    useMemo: vv,
    useReducer: cn,
    useRef: av,
    useState: function() {
      return cn(Kt);
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = Ml();
      return ov(
        a,
        fl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = cn(Kt)[0], t = Ml().memoizedState;
      return [
        typeof l == "boolean" ? l : le(l),
        t
      ];
    },
    useSyncExternalStore: Xs,
    useId: hv,
    useHostTransitionStatus: ni,
    useFormState: Ps,
    useActionState: Ps,
    useOptimistic: function(l, t) {
      var a = Ml();
      return Js(a, fl, l, t);
    },
    useMemoCache: $c,
    useCacheRefresh: gv
  };
  ii.useEffectEvent = ev;
  var zv = {
    readContext: Vl,
    use: nn,
    useCallback: sv,
    useContext: Vl,
    useEffect: ti,
    useImperativeHandle: fv,
    useInsertionEffect: nv,
    useLayoutEffect: cv,
    useMemo: vv,
    useReducer: Ic,
    useRef: av,
    useState: function() {
      return Ic(Kt);
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = Ml();
      return fl === null ? ui(a, l, t) : ov(
        a,
        fl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = Ic(Kt)[0], t = Ml().memoizedState;
      return [
        typeof l == "boolean" ? l : le(l),
        t
      ];
    },
    useSyncExternalStore: Xs,
    useId: hv,
    useHostTransitionStatus: ni,
    useFormState: tv,
    useActionState: tv,
    useOptimistic: function(l, t) {
      var a = Ml();
      return fl !== null ? Js(a, fl, l, t) : (a.baseState = l, [l, a.queue.dispatch]);
    },
    useMemoCache: $c,
    useCacheRefresh: gv
  };
  zv.useEffectEvent = ev;
  function fi(l, t, a, u) {
    t = l.memoizedState, a = a(u, t), a = a == null ? t : x({}, t, a), l.memoizedState = a, l.lanes === 0 && (l.updateQueue.baseState = a);
  }
  var si = {
    enqueueSetState: function(l, t, a) {
      l = l._reactInternals;
      var u = gt(), e = sa(u);
      e.payload = t, a != null && (e.callback = a), t = va(l, e, u), t !== null && (ut(t, l, u), $u(t, l, u));
    },
    enqueueReplaceState: function(l, t, a) {
      l = l._reactInternals;
      var u = gt(), e = sa(u);
      e.tag = 1, e.payload = t, a != null && (e.callback = a), t = va(l, e, u), t !== null && (ut(t, l, u), $u(t, l, u));
    },
    enqueueForceUpdate: function(l, t) {
      l = l._reactInternals;
      var a = gt(), u = sa(a);
      u.tag = 2, t != null && (u.callback = t), t = va(l, u, a), t !== null && (ut(t, l, a), $u(t, l, a));
    }
  };
  function Ev(l, t, a, u, e, n, c) {
    return l = l.stateNode, typeof l.shouldComponentUpdate == "function" ? l.shouldComponentUpdate(u, n, c) : t.prototype && t.prototype.isPureReactComponent ? !Zu(a, u) || !Zu(e, n) : !0;
  }
  function Tv(l, t, a, u) {
    l = t.state, typeof t.componentWillReceiveProps == "function" && t.componentWillReceiveProps(a, u), typeof t.UNSAFE_componentWillReceiveProps == "function" && t.UNSAFE_componentWillReceiveProps(a, u), t.state !== l && si.enqueueReplaceState(t, t.state, null);
  }
  function Za(l, t) {
    var a = t;
    if ("ref" in t) {
      a = {};
      for (var u in t)
        u !== "ref" && (a[u] = t[u]);
    }
    if (l = l.defaultProps) {
      a === t && (a = x({}, a));
      for (var e in l)
        a[e] === void 0 && (a[e] = l[e]);
    }
    return a;
  }
  function Av(l) {
    Qe(l);
  }
  function pv(l) {
    console.error(l);
  }
  function Mv(l) {
    Qe(l);
  }
  function on(l, t) {
    try {
      var a = l.onUncaughtError;
      a(t.value, { componentStack: t.stack });
    } catch (u) {
      setTimeout(function() {
        throw u;
      });
    }
  }
  function Ov(l, t, a) {
    try {
      var u = l.onCaughtError;
      u(a.value, {
        componentStack: a.stack,
        errorBoundary: t.tag === 1 ? t.stateNode : null
      });
    } catch (e) {
      setTimeout(function() {
        throw e;
      });
    }
  }
  function vi(l, t, a) {
    return a = sa(a), a.tag = 3, a.payload = { element: null }, a.callback = function() {
      on(l, t);
    }, a;
  }
  function Dv(l) {
    return l = sa(l), l.tag = 3, l;
  }
  function Uv(l, t, a, u) {
    var e = a.type.getDerivedStateFromError;
    if (typeof e == "function") {
      var n = u.value;
      l.payload = function() {
        return e(n);
      }, l.callback = function() {
        Ov(t, a, u);
      };
    }
    var c = a.stateNode;
    c !== null && typeof c.componentDidCatch == "function" && (l.callback = function() {
      Ov(t, a, u), typeof e != "function" && (ga === null ? ga = /* @__PURE__ */ new Set([this]) : ga.add(this));
      var i = u.stack;
      this.componentDidCatch(u.value, {
        componentStack: i !== null ? i : ""
      });
    });
  }
  function F0(l, t, a, u, e) {
    if (a.flags |= 32768, u !== null && typeof u == "object" && typeof u.then == "function") {
      if (t = a.alternate, t !== null && fu(
        t,
        a,
        e,
        !0
      ), a = dt.current, a !== null) {
        switch (a.tag) {
          case 31:
          case 13:
            return At === null ? Tn() : a.alternate === null && zl === 0 && (zl = 3), a.flags &= -257, a.flags |= 65536, a.lanes = e, u === Fe ? a.flags |= 16384 : (t = a.updateQueue, t === null ? a.updateQueue = /* @__PURE__ */ new Set([u]) : t.add(u), qi(l, u, e)), !1;
          case 22:
            return a.flags |= 65536, u === Fe ? a.flags |= 16384 : (t = a.updateQueue, t === null ? (t = {
              transitions: null,
              markerInstances: null,
              retryQueue: /* @__PURE__ */ new Set([u])
            }, a.updateQueue = t) : (a = t.retryQueue, a === null ? t.retryQueue = /* @__PURE__ */ new Set([u]) : a.add(u)), qi(l, u, e)), !1;
        }
        throw Error(d(435, a.tag));
      }
      return qi(l, u, e), Tn(), !1;
    }
    if ($)
      return t = dt.current, t !== null ? ((t.flags & 65536) === 0 && (t.flags |= 256), t.flags |= 65536, t.lanes = e, u !== Uc && (l = Error(d(422), { cause: u }), Ku(_t(l, a)))) : (u !== Uc && (t = Error(d(423), {
        cause: u
      }), Ku(
        _t(t, a)
      )), l = l.current.alternate, l.flags |= 65536, e &= -e, l.lanes |= e, u = _t(u, a), e = vi(
        l.stateNode,
        u,
        e
      ), Xc(l, e), zl !== 4 && (zl = 2)), !1;
    var n = Error(d(520), { cause: u });
    if (n = _t(n, a), ve === null ? ve = [n] : ve.push(n), zl !== 4 && (zl = 2), t === null) return !0;
    u = _t(u, a), a = t;
    do {
      switch (a.tag) {
        case 3:
          return a.flags |= 65536, l = e & -e, a.lanes |= l, l = vi(a.stateNode, u, l), Xc(a, l), !1;
        case 1:
          if (t = a.type, n = a.stateNode, (a.flags & 128) === 0 && (typeof t.getDerivedStateFromError == "function" || n !== null && typeof n.componentDidCatch == "function" && (ga === null || !ga.has(n))))
            return a.flags |= 65536, e &= -e, a.lanes |= e, e = Dv(e), Uv(
              e,
              l,
              a,
              u
            ), Xc(a, e), !1;
      }
      a = a.return;
    } while (a !== null);
    return !1;
  }
  var oi = Error(d(461)), Ul = !1;
  function Kl(l, t, a, u) {
    t.child = l === null ? Rs(t, null, a, u) : Xa(
      t,
      l.child,
      a,
      u
    );
  }
  function Nv(l, t, a, u, e) {
    a = a.render;
    var n = t.ref;
    if ("ref" in u) {
      var c = {};
      for (var i in u)
        i !== "ref" && (c[i] = u[i]);
    } else c = u;
    return Ba(t), u = Jc(
      l,
      t,
      a,
      c,
      n,
      e
    ), i = wc(), l !== null && !Ul ? (kc(l, t, e), Jt(l, t, e)) : ($ && i && Oc(t), t.flags |= 1, Kl(l, t, u, e), t.child);
  }
  function jv(l, t, a, u, e) {
    if (l === null) {
      var n = a.type;
      return typeof n == "function" && !Ac(n) && n.defaultProps === void 0 && a.compare === null ? (t.tag = 15, t.type = n, Hv(
        l,
        t,
        n,
        u,
        e
      )) : (l = Ke(
        a.type,
        null,
        u,
        t,
        t.mode,
        e
      ), l.ref = t.ref, l.return = t, t.child = l);
    }
    if (n = l.child, !bi(l, e)) {
      var c = n.memoizedProps;
      if (a = a.compare, a = a !== null ? a : Zu, a(c, u) && l.ref === t.ref)
        return Jt(l, t, e);
    }
    return t.flags |= 1, l = Xt(n, u), l.ref = t.ref, l.return = t, t.child = l;
  }
  function Hv(l, t, a, u, e) {
    if (l !== null) {
      var n = l.memoizedProps;
      if (Zu(n, u) && l.ref === t.ref)
        if (Ul = !1, t.pendingProps = u = n, bi(l, e))
          (l.flags & 131072) !== 0 && (Ul = !0);
        else
          return t.lanes = l.lanes, Jt(l, t, e);
    }
    return di(
      l,
      t,
      a,
      u,
      e
    );
  }
  function Rv(l, t, a, u) {
    var e = u.children, n = l !== null ? l.memoizedState : null;
    if (l === null && t.stateNode === null && (t.stateNode = {
      _visibility: 1,
      _pendingMarkers: null,
      _retryCache: null,
      _transitions: null
    }), u.mode === "hidden") {
      if ((t.flags & 128) !== 0) {
        if (n = n !== null ? n.baseLanes | a : a, l !== null) {
          for (u = t.child = l.child, e = 0; u !== null; )
            e = e | u.lanes | u.childLanes, u = u.sibling;
          u = e & ~n;
        } else u = 0, t.child = null;
        return xv(
          l,
          t,
          n,
          a,
          u
        );
      }
      if ((a & 536870912) !== 0)
        t.memoizedState = { baseLanes: 0, cachePool: null }, l !== null && We(
          t,
          n !== null ? n.cachePool : null
        ), n !== null ? Bs(t, n) : Zc(), Cs(t);
      else
        return u = t.lanes = 536870912, xv(
          l,
          t,
          n !== null ? n.baseLanes | a : a,
          a,
          u
        );
    } else
      n !== null ? (We(t, n.cachePool), Bs(t, n), da(), t.memoizedState = null) : (l !== null && We(t, null), Zc(), da());
    return Kl(l, t, e, a), t.child;
  }
  function ue(l, t) {
    return l !== null && l.tag === 22 || t.stateNode !== null || (t.stateNode = {
      _visibility: 1,
      _pendingMarkers: null,
      _retryCache: null,
      _transitions: null
    }), t.sibling;
  }
  function xv(l, t, a, u, e) {
    var n = Bc();
    return n = n === null ? null : { parent: Ol._currentValue, pool: n }, t.memoizedState = {
      baseLanes: a,
      cachePool: n
    }, l !== null && We(t, null), Zc(), Cs(t), l !== null && fu(l, t, u, !0), t.childLanes = e, null;
  }
  function dn(l, t) {
    return t = mn(
      { mode: t.mode, children: t.children },
      l.mode
    ), t.ref = l.ref, l.child = t, t.return = l, t;
  }
  function qv(l, t, a) {
    return Xa(t, l.child, null, a), l = dn(t, t.pendingProps), l.flags |= 2, yt(t), t.memoizedState = null, l;
  }
  function I0(l, t, a) {
    var u = t.pendingProps, e = (t.flags & 128) !== 0;
    if (t.flags &= -129, l === null) {
      if ($) {
        if (u.mode === "hidden")
          return l = dn(t, u), t.lanes = 536870912, ue(null, l);
        if (Vc(t), (l = hl) ? (l = ko(
          l,
          Tt
        ), l = l !== null && l.data === "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ea !== null ? { id: jt, overflow: Ht } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = rs(l), a.return = t, t.child = a, Ll = t, hl = null)) : l = null, l === null) throw ca(t);
        return t.lanes = 536870912, null;
      }
      return dn(t, u);
    }
    var n = l.memoizedState;
    if (n !== null) {
      var c = n.dehydrated;
      if (Vc(t), e)
        if (t.flags & 256)
          t.flags &= -257, t = qv(
            l,
            t,
            a
          );
        else if (t.memoizedState !== null)
          t.child = l.child, t.flags |= 128, t = null;
        else throw Error(d(558));
      else if (Ul || fu(l, t, a, !1), e = (a & l.childLanes) !== 0, Ul || e) {
        if (u = dl, u !== null && (c = pf(u, a), c !== 0 && c !== n.retryLane))
          throw n.retryLane = c, Ha(l, c), ut(u, l, c), oi;
        Tn(), t = qv(
          l,
          t,
          a
        );
      } else
        l = n.treeContext, hl = pt(c.nextSibling), Ll = t, $ = !0, na = null, Tt = !1, l !== null && zs(t, l), t = dn(t, u), t.flags |= 4096;
      return t;
    }
    return l = Xt(l.child, {
      mode: u.mode,
      children: u.children
    }), l.ref = t.ref, t.child = l, l.return = t, l;
  }
  function yn(l, t) {
    var a = t.ref;
    if (a === null)
      l !== null && l.ref !== null && (t.flags |= 4194816);
    else {
      if (typeof a != "function" && typeof a != "object")
        throw Error(d(284));
      (l === null || l.ref !== a) && (t.flags |= 4194816);
    }
  }
  function di(l, t, a, u, e) {
    return Ba(t), a = Jc(
      l,
      t,
      a,
      u,
      void 0,
      e
    ), u = wc(), l !== null && !Ul ? (kc(l, t, e), Jt(l, t, e)) : ($ && u && Oc(t), t.flags |= 1, Kl(l, t, a, e), t.child);
  }
  function Bv(l, t, a, u, e, n) {
    return Ba(t), t.updateQueue = null, a = Gs(
      t,
      u,
      a,
      e
    ), Ys(l), u = wc(), l !== null && !Ul ? (kc(l, t, n), Jt(l, t, n)) : ($ && u && Oc(t), t.flags |= 1, Kl(l, t, a, n), t.child);
  }
  function Cv(l, t, a, u, e) {
    if (Ba(t), t.stateNode === null) {
      var n = eu, c = a.contextType;
      typeof c == "object" && c !== null && (n = Vl(c)), n = new a(u, n), t.memoizedState = n.state !== null && n.state !== void 0 ? n.state : null, n.updater = si, t.stateNode = n, n._reactInternals = t, n = t.stateNode, n.props = u, n.state = t.memoizedState, n.refs = {}, Yc(t), c = a.contextType, n.context = typeof c == "object" && c !== null ? Vl(c) : eu, n.state = t.memoizedState, c = a.getDerivedStateFromProps, typeof c == "function" && (fi(
        t,
        a,
        c,
        u
      ), n.state = t.memoizedState), typeof a.getDerivedStateFromProps == "function" || typeof n.getSnapshotBeforeUpdate == "function" || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (c = n.state, typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount(), c !== n.state && si.enqueueReplaceState(n, n.state, null), Iu(t, u, n, e), Fu(), n.state = t.memoizedState), typeof n.componentDidMount == "function" && (t.flags |= 4194308), u = !0;
    } else if (l === null) {
      n = t.stateNode;
      var i = t.memoizedProps, f = Za(a, i);
      n.props = f;
      var m = n.context, r = a.contextType;
      c = eu, typeof r == "object" && r !== null && (c = Vl(r));
      var z = a.getDerivedStateFromProps;
      r = typeof z == "function" || typeof n.getSnapshotBeforeUpdate == "function", i = t.pendingProps !== i, r || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (i || m !== c) && Tv(
        t,
        n,
        u,
        c
      ), fa = !1;
      var h = t.memoizedState;
      n.state = h, Iu(t, u, n, e), Fu(), m = t.memoizedState, i || h !== m || fa ? (typeof z == "function" && (fi(
        t,
        a,
        z,
        u
      ), m = t.memoizedState), (f = fa || Ev(
        t,
        a,
        f,
        u,
        h,
        m,
        c
      )) ? (r || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount()), typeof n.componentDidMount == "function" && (t.flags |= 4194308)) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), t.memoizedProps = u, t.memoizedState = m), n.props = u, n.state = m, n.context = c, u = f) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), u = !1);
    } else {
      n = t.stateNode, Gc(l, t), c = t.memoizedProps, r = Za(a, c), n.props = r, z = t.pendingProps, h = n.context, m = a.contextType, f = eu, typeof m == "object" && m !== null && (f = Vl(m)), i = a.getDerivedStateFromProps, (m = typeof i == "function" || typeof n.getSnapshotBeforeUpdate == "function") || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (c !== z || h !== f) && Tv(
        t,
        n,
        u,
        f
      ), fa = !1, h = t.memoizedState, n.state = h, Iu(t, u, n, e), Fu();
      var g = t.memoizedState;
      c !== z || h !== g || fa || l !== null && l.dependencies !== null && we(l.dependencies) ? (typeof i == "function" && (fi(
        t,
        a,
        i,
        u
      ), g = t.memoizedState), (r = fa || Ev(
        t,
        a,
        r,
        u,
        h,
        g,
        f
      ) || l !== null && l.dependencies !== null && we(l.dependencies)) ? (m || typeof n.UNSAFE_componentWillUpdate != "function" && typeof n.componentWillUpdate != "function" || (typeof n.componentWillUpdate == "function" && n.componentWillUpdate(u, g, f), typeof n.UNSAFE_componentWillUpdate == "function" && n.UNSAFE_componentWillUpdate(
        u,
        g,
        f
      )), typeof n.componentDidUpdate == "function" && (t.flags |= 4), typeof n.getSnapshotBeforeUpdate == "function" && (t.flags |= 1024)) : (typeof n.componentDidUpdate != "function" || c === l.memoizedProps && h === l.memoizedState || (t.flags |= 4), typeof n.getSnapshotBeforeUpdate != "function" || c === l.memoizedProps && h === l.memoizedState || (t.flags |= 1024), t.memoizedProps = u, t.memoizedState = g), n.props = u, n.state = g, n.context = f, u = r) : (typeof n.componentDidUpdate != "function" || c === l.memoizedProps && h === l.memoizedState || (t.flags |= 4), typeof n.getSnapshotBeforeUpdate != "function" || c === l.memoizedProps && h === l.memoizedState || (t.flags |= 1024), u = !1);
    }
    return n = u, yn(l, t), u = (t.flags & 128) !== 0, n || u ? (n = t.stateNode, a = u && typeof a.getDerivedStateFromError != "function" ? null : n.render(), t.flags |= 1, l !== null && u ? (t.child = Xa(
      t,
      l.child,
      null,
      e
    ), t.child = Xa(
      t,
      null,
      a,
      e
    )) : Kl(l, t, a, e), t.memoizedState = n.state, l = t.child) : l = Jt(
      l,
      t,
      e
    ), l;
  }
  function Yv(l, t, a, u) {
    return xa(), t.flags |= 256, Kl(l, t, a, u), t.child;
  }
  var yi = {
    dehydrated: null,
    treeContext: null,
    retryLane: 0,
    hydrationErrors: null
  };
  function mi(l) {
    return { baseLanes: l, cachePool: Os() };
  }
  function hi(l, t, a) {
    return l = l !== null ? l.childLanes & ~a : 0, t && (l |= ht), l;
  }
  function Gv(l, t, a) {
    var u = t.pendingProps, e = !1, n = (t.flags & 128) !== 0, c;
    if ((c = n) || (c = l !== null && l.memoizedState === null ? !1 : (pl.current & 2) !== 0), c && (e = !0, t.flags &= -129), c = (t.flags & 32) !== 0, t.flags &= -33, l === null) {
      if ($) {
        if (e ? oa(t) : da(), (l = hl) ? (l = ko(
          l,
          Tt
        ), l = l !== null && l.data !== "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ea !== null ? { id: jt, overflow: Ht } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = rs(l), a.return = t, t.child = a, Ll = t, hl = null)) : l = null, l === null) throw ca(t);
        return Fi(l) ? t.lanes = 32 : t.lanes = 536870912, null;
      }
      var i = u.children;
      return u = u.fallback, e ? (da(), e = t.mode, i = mn(
        { mode: "hidden", children: i },
        e
      ), u = Ra(
        u,
        e,
        a,
        null
      ), i.return = t, u.return = t, i.sibling = u, t.child = i, u = t.child, u.memoizedState = mi(a), u.childLanes = hi(
        l,
        c,
        a
      ), t.memoizedState = yi, ue(null, u)) : (oa(t), gi(t, i));
    }
    var f = l.memoizedState;
    if (f !== null && (i = f.dehydrated, i !== null)) {
      if (n)
        t.flags & 256 ? (oa(t), t.flags &= -257, t = Si(
          l,
          t,
          a
        )) : t.memoizedState !== null ? (da(), t.child = l.child, t.flags |= 128, t = null) : (da(), i = u.fallback, e = t.mode, u = mn(
          { mode: "visible", children: u.children },
          e
        ), i = Ra(
          i,
          e,
          a,
          null
        ), i.flags |= 2, u.return = t, i.return = t, u.sibling = i, t.child = u, Xa(
          t,
          l.child,
          null,
          a
        ), u = t.child, u.memoizedState = mi(a), u.childLanes = hi(
          l,
          c,
          a
        ), t.memoizedState = yi, t = ue(null, u));
      else if (oa(t), Fi(i)) {
        if (c = i.nextSibling && i.nextSibling.dataset, c) var m = c.dgst;
        c = m, u = Error(d(419)), u.stack = "", u.digest = c, Ku({ value: u, source: null, stack: null }), t = Si(
          l,
          t,
          a
        );
      } else if (Ul || fu(l, t, a, !1), c = (a & l.childLanes) !== 0, Ul || c) {
        if (c = dl, c !== null && (u = pf(c, a), u !== 0 && u !== f.retryLane))
          throw f.retryLane = u, Ha(l, u), ut(c, l, u), oi;
        $i(i) || Tn(), t = Si(
          l,
          t,
          a
        );
      } else
        $i(i) ? (t.flags |= 192, t.child = l.child, t = null) : (l = f.treeContext, hl = pt(
          i.nextSibling
        ), Ll = t, $ = !0, na = null, Tt = !1, l !== null && zs(t, l), t = gi(
          t,
          u.children
        ), t.flags |= 4096);
      return t;
    }
    return e ? (da(), i = u.fallback, e = t.mode, f = l.child, m = f.sibling, u = Xt(f, {
      mode: "hidden",
      children: u.children
    }), u.subtreeFlags = f.subtreeFlags & 65011712, m !== null ? i = Xt(
      m,
      i
    ) : (i = Ra(
      i,
      e,
      a,
      null
    ), i.flags |= 2), i.return = t, u.return = t, u.sibling = i, t.child = u, ue(null, u), u = t.child, i = l.child.memoizedState, i === null ? i = mi(a) : (e = i.cachePool, e !== null ? (f = Ol._currentValue, e = e.parent !== f ? { parent: f, pool: f } : e) : e = Os(), i = {
      baseLanes: i.baseLanes | a,
      cachePool: e
    }), u.memoizedState = i, u.childLanes = hi(
      l,
      c,
      a
    ), t.memoizedState = yi, ue(l.child, u)) : (oa(t), a = l.child, l = a.sibling, a = Xt(a, {
      mode: "visible",
      children: u.children
    }), a.return = t, a.sibling = null, l !== null && (c = t.deletions, c === null ? (t.deletions = [l], t.flags |= 16) : c.push(l)), t.child = a, t.memoizedState = null, a);
  }
  function gi(l, t) {
    return t = mn(
      { mode: "visible", children: t },
      l.mode
    ), t.return = l, l.child = t;
  }
  function mn(l, t) {
    return l = ot(22, l, null, t), l.lanes = 0, l;
  }
  function Si(l, t, a) {
    return Xa(t, l.child, null, a), l = gi(
      t,
      t.pendingProps.children
    ), l.flags |= 2, t.memoizedState = null, l;
  }
  function Xv(l, t, a) {
    l.lanes |= t;
    var u = l.alternate;
    u !== null && (u.lanes |= t), Hc(l.return, t, a);
  }
  function ri(l, t, a, u, e, n) {
    var c = l.memoizedState;
    c === null ? l.memoizedState = {
      isBackwards: t,
      rendering: null,
      renderingStartTime: 0,
      last: u,
      tail: a,
      tailMode: e,
      treeForkCount: n
    } : (c.isBackwards = t, c.rendering = null, c.renderingStartTime = 0, c.last = u, c.tail = a, c.tailMode = e, c.treeForkCount = n);
  }
  function Qv(l, t, a) {
    var u = t.pendingProps, e = u.revealOrder, n = u.tail;
    u = u.children;
    var c = pl.current, i = (c & 2) !== 0;
    if (i ? (c = c & 1 | 2, t.flags |= 128) : c &= 1, O(pl, c), Kl(l, t, u, a), u = $ ? Vu : 0, !i && l !== null && (l.flags & 128) !== 0)
      l: for (l = t.child; l !== null; ) {
        if (l.tag === 13)
          l.memoizedState !== null && Xv(l, a, t);
        else if (l.tag === 19)
          Xv(l, a, t);
        else if (l.child !== null) {
          l.child.return = l, l = l.child;
          continue;
        }
        if (l === t) break l;
        for (; l.sibling === null; ) {
          if (l.return === null || l.return === t)
            break l;
          l = l.return;
        }
        l.sibling.return = l.return, l = l.sibling;
      }
    switch (e) {
      case "forwards":
        for (a = t.child, e = null; a !== null; )
          l = a.alternate, l !== null && tn(l) === null && (e = a), a = a.sibling;
        a = e, a === null ? (e = t.child, t.child = null) : (e = a.sibling, a.sibling = null), ri(
          t,
          !1,
          e,
          a,
          n,
          u
        );
        break;
      case "backwards":
      case "unstable_legacy-backwards":
        for (a = null, e = t.child, t.child = null; e !== null; ) {
          if (l = e.alternate, l !== null && tn(l) === null) {
            t.child = e;
            break;
          }
          l = e.sibling, e.sibling = a, a = e, e = l;
        }
        ri(
          t,
          !0,
          a,
          null,
          n,
          u
        );
        break;
      case "together":
        ri(
          t,
          !1,
          null,
          null,
          void 0,
          u
        );
        break;
      default:
        t.memoizedState = null;
    }
    return t.child;
  }
  function Jt(l, t, a) {
    if (l !== null && (t.dependencies = l.dependencies), ha |= t.lanes, (a & t.childLanes) === 0)
      if (l !== null) {
        if (fu(
          l,
          t,
          a,
          !1
        ), (a & t.childLanes) === 0)
          return null;
      } else return null;
    if (l !== null && t.child !== l.child)
      throw Error(d(153));
    if (t.child !== null) {
      for (l = t.child, a = Xt(l, l.pendingProps), t.child = a, a.return = t; l.sibling !== null; )
        l = l.sibling, a = a.sibling = Xt(l, l.pendingProps), a.return = t;
      a.sibling = null;
    }
    return t.child;
  }
  function bi(l, t) {
    return (l.lanes & t) !== 0 ? !0 : (l = l.dependencies, !!(l !== null && we(l)));
  }
  function P0(l, t, a) {
    switch (t.tag) {
      case 3:
        Wl(t, t.stateNode.containerInfo), ia(t, Ol, l.memoizedState.cache), xa();
        break;
      case 27:
      case 5:
        Uu(t);
        break;
      case 4:
        Wl(t, t.stateNode.containerInfo);
        break;
      case 10:
        ia(
          t,
          t.type,
          t.memoizedProps.value
        );
        break;
      case 31:
        if (t.memoizedState !== null)
          return t.flags |= 128, Vc(t), null;
        break;
      case 13:
        var u = t.memoizedState;
        if (u !== null)
          return u.dehydrated !== null ? (oa(t), t.flags |= 128, null) : (a & t.child.childLanes) !== 0 ? Gv(l, t, a) : (oa(t), l = Jt(
            l,
            t,
            a
          ), l !== null ? l.sibling : null);
        oa(t);
        break;
      case 19:
        var e = (l.flags & 128) !== 0;
        if (u = (a & t.childLanes) !== 0, u || (fu(
          l,
          t,
          a,
          !1
        ), u = (a & t.childLanes) !== 0), e) {
          if (u)
            return Qv(
              l,
              t,
              a
            );
          t.flags |= 128;
        }
        if (e = t.memoizedState, e !== null && (e.rendering = null, e.tail = null, e.lastEffect = null), O(pl, pl.current), u) break;
        return null;
      case 22:
        return t.lanes = 0, Rv(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        ia(t, Ol, l.memoizedState.cache);
    }
    return Jt(l, t, a);
  }
  function Zv(l, t, a) {
    if (l !== null)
      if (l.memoizedProps !== t.pendingProps)
        Ul = !0;
      else {
        if (!bi(l, a) && (t.flags & 128) === 0)
          return Ul = !1, P0(
            l,
            t,
            a
          );
        Ul = (l.flags & 131072) !== 0;
      }
    else
      Ul = !1, $ && (t.flags & 1048576) !== 0 && _s(t, Vu, t.index);
    switch (t.lanes = 0, t.tag) {
      case 16:
        l: {
          var u = t.pendingProps;
          if (l = Ya(t.elementType), t.type = l, typeof l == "function")
            Ac(l) ? (u = Za(l, u), t.tag = 1, t = Cv(
              null,
              t,
              l,
              u,
              a
            )) : (t.tag = 0, t = di(
              null,
              t,
              l,
              u,
              a
            ));
          else {
            if (l != null) {
              var e = l.$$typeof;
              if (e === kl) {
                t.tag = 11, t = Nv(
                  null,
                  t,
                  l,
                  u,
                  a
                );
                break l;
              } else if (e === V) {
                t.tag = 14, t = jv(
                  null,
                  t,
                  l,
                  u,
                  a
                );
                break l;
              }
            }
            throw t = j(l) || l, Error(d(306, t, ""));
          }
        }
        return t;
      case 0:
        return di(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 1:
        return u = t.type, e = Za(
          u,
          t.pendingProps
        ), Cv(
          l,
          t,
          u,
          e,
          a
        );
      case 3:
        l: {
          if (Wl(
            t,
            t.stateNode.containerInfo
          ), l === null) throw Error(d(387));
          u = t.pendingProps;
          var n = t.memoizedState;
          e = n.element, Gc(l, t), Iu(t, u, null, a);
          var c = t.memoizedState;
          if (u = c.cache, ia(t, Ol, u), u !== n.cache && Rc(
            t,
            [Ol],
            a,
            !0
          ), Fu(), u = c.element, n.isDehydrated)
            if (n = {
              element: u,
              isDehydrated: !1,
              cache: c.cache
            }, t.updateQueue.baseState = n, t.memoizedState = n, t.flags & 256) {
              t = Yv(
                l,
                t,
                u,
                a
              );
              break l;
            } else if (u !== e) {
              e = _t(
                Error(d(424)),
                t
              ), Ku(e), t = Yv(
                l,
                t,
                u,
                a
              );
              break l;
            } else {
              switch (l = t.stateNode.containerInfo, l.nodeType) {
                case 9:
                  l = l.body;
                  break;
                default:
                  l = l.nodeName === "HTML" ? l.ownerDocument.body : l;
              }
              for (hl = pt(l.firstChild), Ll = t, $ = !0, na = null, Tt = !0, a = Rs(
                t,
                null,
                u,
                a
              ), t.child = a; a; )
                a.flags = a.flags & -3 | 4096, a = a.sibling;
            }
          else {
            if (xa(), u === e) {
              t = Jt(
                l,
                t,
                a
              );
              break l;
            }
            Kl(l, t, u, a);
          }
          t = t.child;
        }
        return t;
      case 26:
        return yn(l, t), l === null ? (a = ld(
          t.type,
          null,
          t.pendingProps,
          null
        )) ? t.memoizedState = a : $ || (a = t.type, l = t.pendingProps, u = Nn(
          K.current
        ).createElement(a), u[Zl] = t, u[Fl] = l, Jl(u, a, l), ql(u), t.stateNode = u) : t.memoizedState = ld(
          t.type,
          l.memoizedProps,
          t.pendingProps,
          l.memoizedState
        ), null;
      case 27:
        return Uu(t), l === null && $ && (u = t.stateNode = Fo(
          t.type,
          t.pendingProps,
          K.current
        ), Ll = t, Tt = !0, e = hl, _a(t.type) ? (Ii = e, hl = pt(u.firstChild)) : hl = e), Kl(
          l,
          t,
          t.pendingProps.children,
          a
        ), yn(l, t), l === null && (t.flags |= 4194304), t.child;
      case 5:
        return l === null && $ && ((e = u = hl) && (u = Uy(
          u,
          t.type,
          t.pendingProps,
          Tt
        ), u !== null ? (t.stateNode = u, Ll = t, hl = pt(u.firstChild), Tt = !1, e = !0) : e = !1), e || ca(t)), Uu(t), e = t.type, n = t.pendingProps, c = l !== null ? l.memoizedProps : null, u = n.children, wi(e, n) ? u = null : c !== null && wi(e, c) && (t.flags |= 32), t.memoizedState !== null && (e = Jc(
          l,
          t,
          V0,
          null,
          null,
          a
        ), re._currentValue = e), yn(l, t), Kl(l, t, u, a), t.child;
      case 6:
        return l === null && $ && ((l = a = hl) && (a = Ny(
          a,
          t.pendingProps,
          Tt
        ), a !== null ? (t.stateNode = a, Ll = t, hl = null, l = !0) : l = !1), l || ca(t)), null;
      case 13:
        return Gv(l, t, a);
      case 4:
        return Wl(
          t,
          t.stateNode.containerInfo
        ), u = t.pendingProps, l === null ? t.child = Xa(
          t,
          null,
          u,
          a
        ) : Kl(l, t, u, a), t.child;
      case 11:
        return Nv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 7:
        return Kl(
          l,
          t,
          t.pendingProps,
          a
        ), t.child;
      case 8:
        return Kl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 12:
        return Kl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 10:
        return u = t.pendingProps, ia(t, t.type, u.value), Kl(l, t, u.children, a), t.child;
      case 9:
        return e = t.type._context, u = t.pendingProps.children, Ba(t), e = Vl(e), u = u(e), t.flags |= 1, Kl(l, t, u, a), t.child;
      case 14:
        return jv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 15:
        return Hv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 19:
        return Qv(l, t, a);
      case 31:
        return I0(l, t, a);
      case 22:
        return Rv(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        return Ba(t), u = Vl(Ol), l === null ? (e = Bc(), e === null && (e = dl, n = xc(), e.pooledCache = n, n.refCount++, n !== null && (e.pooledCacheLanes |= a), e = n), t.memoizedState = { parent: u, cache: e }, Yc(t), ia(t, Ol, e)) : ((l.lanes & a) !== 0 && (Gc(l, t), Iu(t, null, null, a), Fu()), e = l.memoizedState, n = t.memoizedState, e.parent !== u ? (e = { parent: u, cache: u }, t.memoizedState = e, t.lanes === 0 && (t.memoizedState = t.updateQueue.baseState = e), ia(t, Ol, u)) : (u = n.cache, ia(t, Ol, u), u !== e.cache && Rc(
          t,
          [Ol],
          a,
          !0
        ))), Kl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 29:
        throw t.pendingProps;
    }
    throw Error(d(156, t.tag));
  }
  function wt(l) {
    l.flags |= 4;
  }
  function _i(l, t, a, u, e) {
    if ((t = (l.mode & 32) !== 0) && (t = !1), t) {
      if (l.flags |= 16777216, (e & 335544128) === e)
        if (l.stateNode.complete) l.flags |= 8192;
        else if (go()) l.flags |= 8192;
        else
          throw Ga = Fe, Cc;
    } else l.flags &= -16777217;
  }
  function Lv(l, t) {
    if (t.type !== "stylesheet" || (t.state.loading & 4) !== 0)
      l.flags &= -16777217;
    else if (l.flags |= 16777216, !nd(t))
      if (go()) l.flags |= 8192;
      else
        throw Ga = Fe, Cc;
  }
  function hn(l, t) {
    t !== null && (l.flags |= 4), l.flags & 16384 && (t = l.tag !== 22 ? Ef() : 536870912, l.lanes |= t, _u |= t);
  }
  function ee(l, t) {
    if (!$)
      switch (l.tailMode) {
        case "hidden":
          t = l.tail;
          for (var a = null; t !== null; )
            t.alternate !== null && (a = t), t = t.sibling;
          a === null ? l.tail = null : a.sibling = null;
          break;
        case "collapsed":
          a = l.tail;
          for (var u = null; a !== null; )
            a.alternate !== null && (u = a), a = a.sibling;
          u === null ? t || l.tail === null ? l.tail = null : l.tail.sibling = null : u.sibling = null;
      }
  }
  function gl(l) {
    var t = l.alternate !== null && l.alternate.child === l.child, a = 0, u = 0;
    if (t)
      for (var e = l.child; e !== null; )
        a |= e.lanes | e.childLanes, u |= e.subtreeFlags & 65011712, u |= e.flags & 65011712, e.return = l, e = e.sibling;
    else
      for (e = l.child; e !== null; )
        a |= e.lanes | e.childLanes, u |= e.subtreeFlags, u |= e.flags, e.return = l, e = e.sibling;
    return l.subtreeFlags |= u, l.childLanes = a, t;
  }
  function ly(l, t, a) {
    var u = t.pendingProps;
    switch (Dc(t), t.tag) {
      case 16:
      case 15:
      case 0:
      case 11:
      case 7:
      case 8:
      case 12:
      case 9:
      case 14:
        return gl(t), null;
      case 1:
        return gl(t), null;
      case 3:
        return a = t.stateNode, u = null, l !== null && (u = l.memoizedState.cache), t.memoizedState.cache !== u && (t.flags |= 2048), Lt(Ol), Al(), a.pendingContext && (a.context = a.pendingContext, a.pendingContext = null), (l === null || l.child === null) && (iu(t) ? wt(t) : l === null || l.memoizedState.isDehydrated && (t.flags & 256) === 0 || (t.flags |= 1024, Nc())), gl(t), null;
      case 26:
        var e = t.type, n = t.memoizedState;
        return l === null ? (wt(t), n !== null ? (gl(t), Lv(t, n)) : (gl(t), _i(
          t,
          e,
          null,
          u,
          a
        ))) : n ? n !== l.memoizedState ? (wt(t), gl(t), Lv(t, n)) : (gl(t), t.flags &= -16777217) : (l = l.memoizedProps, l !== u && wt(t), gl(t), _i(
          t,
          e,
          l,
          u,
          a
        )), null;
      case 27:
        if (pe(t), a = K.current, e = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== u && wt(t);
        else {
          if (!u) {
            if (t.stateNode === null)
              throw Error(d(166));
            return gl(t), null;
          }
          l = H.current, iu(t) ? Es(t) : (l = Fo(e, u, a), t.stateNode = l, wt(t));
        }
        return gl(t), null;
      case 5:
        if (pe(t), e = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== u && wt(t);
        else {
          if (!u) {
            if (t.stateNode === null)
              throw Error(d(166));
            return gl(t), null;
          }
          if (n = H.current, iu(t))
            Es(t);
          else {
            var c = Nn(
              K.current
            );
            switch (n) {
              case 1:
                n = c.createElementNS(
                  "http://www.w3.org/2000/svg",
                  e
                );
                break;
              case 2:
                n = c.createElementNS(
                  "http://www.w3.org/1998/Math/MathML",
                  e
                );
                break;
              default:
                switch (e) {
                  case "svg":
                    n = c.createElementNS(
                      "http://www.w3.org/2000/svg",
                      e
                    );
                    break;
                  case "math":
                    n = c.createElementNS(
                      "http://www.w3.org/1998/Math/MathML",
                      e
                    );
                    break;
                  case "script":
                    n = c.createElement("div"), n.innerHTML = "<script><\/script>", n = n.removeChild(
                      n.firstChild
                    );
                    break;
                  case "select":
                    n = typeof u.is == "string" ? c.createElement("select", {
                      is: u.is
                    }) : c.createElement("select"), u.multiple ? n.multiple = !0 : u.size && (n.size = u.size);
                    break;
                  default:
                    n = typeof u.is == "string" ? c.createElement(e, { is: u.is }) : c.createElement(e);
                }
            }
            n[Zl] = t, n[Fl] = u;
            l: for (c = t.child; c !== null; ) {
              if (c.tag === 5 || c.tag === 6)
                n.appendChild(c.stateNode);
              else if (c.tag !== 4 && c.tag !== 27 && c.child !== null) {
                c.child.return = c, c = c.child;
                continue;
              }
              if (c === t) break l;
              for (; c.sibling === null; ) {
                if (c.return === null || c.return === t)
                  break l;
                c = c.return;
              }
              c.sibling.return = c.return, c = c.sibling;
            }
            t.stateNode = n;
            l: switch (Jl(n, e, u), e) {
              case "button":
              case "input":
              case "select":
              case "textarea":
                u = !!u.autoFocus;
                break l;
              case "img":
                u = !0;
                break l;
              default:
                u = !1;
            }
            u && wt(t);
          }
        }
        return gl(t), _i(
          t,
          t.type,
          l === null ? null : l.memoizedProps,
          t.pendingProps,
          a
        ), null;
      case 6:
        if (l && t.stateNode != null)
          l.memoizedProps !== u && wt(t);
        else {
          if (typeof u != "string" && t.stateNode === null)
            throw Error(d(166));
          if (l = K.current, iu(t)) {
            if (l = t.stateNode, a = t.memoizedProps, u = null, e = Ll, e !== null)
              switch (e.tag) {
                case 27:
                case 5:
                  u = e.memoizedProps;
              }
            l[Zl] = t, l = !!(l.nodeValue === a || u !== null && u.suppressHydrationWarning === !0 || Xo(l.nodeValue, a)), l || ca(t, !0);
          } else
            l = Nn(l).createTextNode(
              u
            ), l[Zl] = t, t.stateNode = l;
        }
        return gl(t), null;
      case 31:
        if (a = t.memoizedState, l === null || l.memoizedState !== null) {
          if (u = iu(t), a !== null) {
            if (l === null) {
              if (!u) throw Error(d(318));
              if (l = t.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(d(557));
              l[Zl] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            gl(t), l = !1;
          } else
            a = Nc(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = a), l = !0;
          if (!l)
            return t.flags & 256 ? (yt(t), t) : (yt(t), null);
          if ((t.flags & 128) !== 0)
            throw Error(d(558));
        }
        return gl(t), null;
      case 13:
        if (u = t.memoizedState, l === null || l.memoizedState !== null && l.memoizedState.dehydrated !== null) {
          if (e = iu(t), u !== null && u.dehydrated !== null) {
            if (l === null) {
              if (!e) throw Error(d(318));
              if (e = t.memoizedState, e = e !== null ? e.dehydrated : null, !e) throw Error(d(317));
              e[Zl] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            gl(t), e = !1;
          } else
            e = Nc(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = e), e = !0;
          if (!e)
            return t.flags & 256 ? (yt(t), t) : (yt(t), null);
        }
        return yt(t), (t.flags & 128) !== 0 ? (t.lanes = a, t) : (a = u !== null, l = l !== null && l.memoizedState !== null, a && (u = t.child, e = null, u.alternate !== null && u.alternate.memoizedState !== null && u.alternate.memoizedState.cachePool !== null && (e = u.alternate.memoizedState.cachePool.pool), n = null, u.memoizedState !== null && u.memoizedState.cachePool !== null && (n = u.memoizedState.cachePool.pool), n !== e && (u.flags |= 2048)), a !== l && a && (t.child.flags |= 8192), hn(t, t.updateQueue), gl(t), null);
      case 4:
        return Al(), l === null && Zi(t.stateNode.containerInfo), gl(t), null;
      case 10:
        return Lt(t.type), gl(t), null;
      case 19:
        if (E(pl), u = t.memoizedState, u === null) return gl(t), null;
        if (e = (t.flags & 128) !== 0, n = u.rendering, n === null)
          if (e) ee(u, !1);
          else {
            if (zl !== 0 || l !== null && (l.flags & 128) !== 0)
              for (l = t.child; l !== null; ) {
                if (n = tn(l), n !== null) {
                  for (t.flags |= 128, ee(u, !1), l = n.updateQueue, t.updateQueue = l, hn(t, l), t.subtreeFlags = 0, l = a, a = t.child; a !== null; )
                    Ss(a, l), a = a.sibling;
                  return O(
                    pl,
                    pl.current & 1 | 2
                  ), $ && Qt(t, u.treeForkCount), t.child;
                }
                l = l.sibling;
              }
            u.tail !== null && it() > _n && (t.flags |= 128, e = !0, ee(u, !1), t.lanes = 4194304);
          }
        else {
          if (!e)
            if (l = tn(n), l !== null) {
              if (t.flags |= 128, e = !0, l = l.updateQueue, t.updateQueue = l, hn(t, l), ee(u, !0), u.tail === null && u.tailMode === "hidden" && !n.alternate && !$)
                return gl(t), null;
            } else
              2 * it() - u.renderingStartTime > _n && a !== 536870912 && (t.flags |= 128, e = !0, ee(u, !1), t.lanes = 4194304);
          u.isBackwards ? (n.sibling = t.child, t.child = n) : (l = u.last, l !== null ? l.sibling = n : t.child = n, u.last = n);
        }
        return u.tail !== null ? (l = u.tail, u.rendering = l, u.tail = l.sibling, u.renderingStartTime = it(), l.sibling = null, a = pl.current, O(
          pl,
          e ? a & 1 | 2 : a & 1
        ), $ && Qt(t, u.treeForkCount), l) : (gl(t), null);
      case 22:
      case 23:
        return yt(t), Lc(), u = t.memoizedState !== null, l !== null ? l.memoizedState !== null !== u && (t.flags |= 8192) : u && (t.flags |= 8192), u ? (a & 536870912) !== 0 && (t.flags & 128) === 0 && (gl(t), t.subtreeFlags & 6 && (t.flags |= 8192)) : gl(t), a = t.updateQueue, a !== null && hn(t, a.retryQueue), a = null, l !== null && l.memoizedState !== null && l.memoizedState.cachePool !== null && (a = l.memoizedState.cachePool.pool), u = null, t.memoizedState !== null && t.memoizedState.cachePool !== null && (u = t.memoizedState.cachePool.pool), u !== a && (t.flags |= 2048), l !== null && E(Ca), null;
      case 24:
        return a = null, l !== null && (a = l.memoizedState.cache), t.memoizedState.cache !== a && (t.flags |= 2048), Lt(Ol), gl(t), null;
      case 25:
        return null;
      case 30:
        return null;
    }
    throw Error(d(156, t.tag));
  }
  function ty(l, t) {
    switch (Dc(t), t.tag) {
      case 1:
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 3:
        return Lt(Ol), Al(), l = t.flags, (l & 65536) !== 0 && (l & 128) === 0 ? (t.flags = l & -65537 | 128, t) : null;
      case 26:
      case 27:
      case 5:
        return pe(t), null;
      case 31:
        if (t.memoizedState !== null) {
          if (yt(t), t.alternate === null)
            throw Error(d(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 13:
        if (yt(t), l = t.memoizedState, l !== null && l.dehydrated !== null) {
          if (t.alternate === null)
            throw Error(d(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 19:
        return E(pl), null;
      case 4:
        return Al(), null;
      case 10:
        return Lt(t.type), null;
      case 22:
      case 23:
        return yt(t), Lc(), l !== null && E(Ca), l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 24:
        return Lt(Ol), null;
      case 25:
        return null;
      default:
        return null;
    }
  }
  function Vv(l, t) {
    switch (Dc(t), t.tag) {
      case 3:
        Lt(Ol), Al();
        break;
      case 26:
      case 27:
      case 5:
        pe(t);
        break;
      case 4:
        Al();
        break;
      case 31:
        t.memoizedState !== null && yt(t);
        break;
      case 13:
        yt(t);
        break;
      case 19:
        E(pl);
        break;
      case 10:
        Lt(t.type);
        break;
      case 22:
      case 23:
        yt(t), Lc(), l !== null && E(Ca);
        break;
      case 24:
        Lt(Ol);
    }
  }
  function ne(l, t) {
    try {
      var a = t.updateQueue, u = a !== null ? a.lastEffect : null;
      if (u !== null) {
        var e = u.next;
        a = e;
        do {
          if ((a.tag & l) === l) {
            u = void 0;
            var n = a.create, c = a.inst;
            u = n(), c.destroy = u;
          }
          a = a.next;
        } while (a !== e);
      }
    } catch (i) {
      nl(t, t.return, i);
    }
  }
  function ya(l, t, a) {
    try {
      var u = t.updateQueue, e = u !== null ? u.lastEffect : null;
      if (e !== null) {
        var n = e.next;
        u = n;
        do {
          if ((u.tag & l) === l) {
            var c = u.inst, i = c.destroy;
            if (i !== void 0) {
              c.destroy = void 0, e = t;
              var f = a, m = i;
              try {
                m();
              } catch (r) {
                nl(
                  e,
                  f,
                  r
                );
              }
            }
          }
          u = u.next;
        } while (u !== n);
      }
    } catch (r) {
      nl(t, t.return, r);
    }
  }
  function Kv(l) {
    var t = l.updateQueue;
    if (t !== null) {
      var a = l.stateNode;
      try {
        qs(t, a);
      } catch (u) {
        nl(l, l.return, u);
      }
    }
  }
  function Jv(l, t, a) {
    a.props = Za(
      l.type,
      l.memoizedProps
    ), a.state = l.memoizedState;
    try {
      a.componentWillUnmount();
    } catch (u) {
      nl(l, t, u);
    }
  }
  function ce(l, t) {
    try {
      var a = l.ref;
      if (a !== null) {
        switch (l.tag) {
          case 26:
          case 27:
          case 5:
            var u = l.stateNode;
            break;
          case 30:
            u = l.stateNode;
            break;
          default:
            u = l.stateNode;
        }
        typeof a == "function" ? l.refCleanup = a(u) : a.current = u;
      }
    } catch (e) {
      nl(l, t, e);
    }
  }
  function Rt(l, t) {
    var a = l.ref, u = l.refCleanup;
    if (a !== null)
      if (typeof u == "function")
        try {
          u();
        } catch (e) {
          nl(l, t, e);
        } finally {
          l.refCleanup = null, l = l.alternate, l != null && (l.refCleanup = null);
        }
      else if (typeof a == "function")
        try {
          a(null);
        } catch (e) {
          nl(l, t, e);
        }
      else a.current = null;
  }
  function wv(l) {
    var t = l.type, a = l.memoizedProps, u = l.stateNode;
    try {
      l: switch (t) {
        case "button":
        case "input":
        case "select":
        case "textarea":
          a.autoFocus && u.focus();
          break l;
        case "img":
          a.src ? u.src = a.src : a.srcSet && (u.srcset = a.srcSet);
      }
    } catch (e) {
      nl(l, l.return, e);
    }
  }
  function zi(l, t, a) {
    try {
      var u = l.stateNode;
      Ty(u, l.type, a, t), u[Fl] = t;
    } catch (e) {
      nl(l, l.return, e);
    }
  }
  function kv(l) {
    return l.tag === 5 || l.tag === 3 || l.tag === 26 || l.tag === 27 && _a(l.type) || l.tag === 4;
  }
  function Ei(l) {
    l: for (; ; ) {
      for (; l.sibling === null; ) {
        if (l.return === null || kv(l.return)) return null;
        l = l.return;
      }
      for (l.sibling.return = l.return, l = l.sibling; l.tag !== 5 && l.tag !== 6 && l.tag !== 18; ) {
        if (l.tag === 27 && _a(l.type) || l.flags & 2 || l.child === null || l.tag === 4) continue l;
        l.child.return = l, l = l.child;
      }
      if (!(l.flags & 2)) return l.stateNode;
    }
  }
  function Ti(l, t, a) {
    var u = l.tag;
    if (u === 5 || u === 6)
      l = l.stateNode, t ? (a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a).insertBefore(l, t) : (t = a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a, t.appendChild(l), a = a._reactRootContainer, a != null || t.onclick !== null || (t.onclick = Yt));
    else if (u !== 4 && (u === 27 && _a(l.type) && (a = l.stateNode, t = null), l = l.child, l !== null))
      for (Ti(l, t, a), l = l.sibling; l !== null; )
        Ti(l, t, a), l = l.sibling;
  }
  function gn(l, t, a) {
    var u = l.tag;
    if (u === 5 || u === 6)
      l = l.stateNode, t ? a.insertBefore(l, t) : a.appendChild(l);
    else if (u !== 4 && (u === 27 && _a(l.type) && (a = l.stateNode), l = l.child, l !== null))
      for (gn(l, t, a), l = l.sibling; l !== null; )
        gn(l, t, a), l = l.sibling;
  }
  function Wv(l) {
    var t = l.stateNode, a = l.memoizedProps;
    try {
      for (var u = l.type, e = t.attributes; e.length; )
        t.removeAttributeNode(e[0]);
      Jl(t, u, a), t[Zl] = l, t[Fl] = a;
    } catch (n) {
      nl(l, l.return, n);
    }
  }
  var kt = !1, Nl = !1, Ai = !1, $v = typeof WeakSet == "function" ? WeakSet : Set, Bl = null;
  function ay(l, t) {
    if (l = l.containerInfo, Ki = Cn, l = fs(l), Sc(l)) {
      if ("selectionStart" in l)
        var a = {
          start: l.selectionStart,
          end: l.selectionEnd
        };
      else
        l: {
          a = (a = l.ownerDocument) && a.defaultView || window;
          var u = a.getSelection && a.getSelection();
          if (u && u.rangeCount !== 0) {
            a = u.anchorNode;
            var e = u.anchorOffset, n = u.focusNode;
            u = u.focusOffset;
            try {
              a.nodeType, n.nodeType;
            } catch {
              a = null;
              break l;
            }
            var c = 0, i = -1, f = -1, m = 0, r = 0, z = l, h = null;
            t: for (; ; ) {
              for (var g; z !== a || e !== 0 && z.nodeType !== 3 || (i = c + e), z !== n || u !== 0 && z.nodeType !== 3 || (f = c + u), z.nodeType === 3 && (c += z.nodeValue.length), (g = z.firstChild) !== null; )
                h = z, z = g;
              for (; ; ) {
                if (z === l) break t;
                if (h === a && ++m === e && (i = c), h === n && ++r === u && (f = c), (g = z.nextSibling) !== null) break;
                z = h, h = z.parentNode;
              }
              z = g;
            }
            a = i === -1 || f === -1 ? null : { start: i, end: f };
          } else a = null;
        }
      a = a || { start: 0, end: 0 };
    } else a = null;
    for (Ji = { focusedElem: l, selectionRange: a }, Cn = !1, Bl = t; Bl !== null; )
      if (t = Bl, l = t.child, (t.subtreeFlags & 1028) !== 0 && l !== null)
        l.return = t, Bl = l;
      else
        for (; Bl !== null; ) {
          switch (t = Bl, n = t.alternate, l = t.flags, t.tag) {
            case 0:
              if ((l & 4) !== 0 && (l = t.updateQueue, l = l !== null ? l.events : null, l !== null))
                for (a = 0; a < l.length; a++)
                  e = l[a], e.ref.impl = e.nextImpl;
              break;
            case 11:
            case 15:
              break;
            case 1:
              if ((l & 1024) !== 0 && n !== null) {
                l = void 0, a = t, e = n.memoizedProps, n = n.memoizedState, u = a.stateNode;
                try {
                  var N = Za(
                    a.type,
                    e
                  );
                  l = u.getSnapshotBeforeUpdate(
                    N,
                    n
                  ), u.__reactInternalSnapshotBeforeUpdate = l;
                } catch (B) {
                  nl(
                    a,
                    a.return,
                    B
                  );
                }
              }
              break;
            case 3:
              if ((l & 1024) !== 0) {
                if (l = t.stateNode.containerInfo, a = l.nodeType, a === 9)
                  Wi(l);
                else if (a === 1)
                  switch (l.nodeName) {
                    case "HEAD":
                    case "HTML":
                    case "BODY":
                      Wi(l);
                      break;
                    default:
                      l.textContent = "";
                  }
              }
              break;
            case 5:
            case 26:
            case 27:
            case 6:
            case 4:
            case 17:
              break;
            default:
              if ((l & 1024) !== 0) throw Error(d(163));
          }
          if (l = t.sibling, l !== null) {
            l.return = t.return, Bl = l;
            break;
          }
          Bl = t.return;
        }
  }
  function Fv(l, t, a) {
    var u = a.flags;
    switch (a.tag) {
      case 0:
      case 11:
      case 15:
        $t(l, a), u & 4 && ne(5, a);
        break;
      case 1:
        if ($t(l, a), u & 4)
          if (l = a.stateNode, t === null)
            try {
              l.componentDidMount();
            } catch (c) {
              nl(a, a.return, c);
            }
          else {
            var e = Za(
              a.type,
              t.memoizedProps
            );
            t = t.memoizedState;
            try {
              l.componentDidUpdate(
                e,
                t,
                l.__reactInternalSnapshotBeforeUpdate
              );
            } catch (c) {
              nl(
                a,
                a.return,
                c
              );
            }
          }
        u & 64 && Kv(a), u & 512 && ce(a, a.return);
        break;
      case 3:
        if ($t(l, a), u & 64 && (l = a.updateQueue, l !== null)) {
          if (t = null, a.child !== null)
            switch (a.child.tag) {
              case 27:
              case 5:
                t = a.child.stateNode;
                break;
              case 1:
                t = a.child.stateNode;
            }
          try {
            qs(l, t);
          } catch (c) {
            nl(a, a.return, c);
          }
        }
        break;
      case 27:
        t === null && u & 4 && Wv(a);
      case 26:
      case 5:
        $t(l, a), t === null && u & 4 && wv(a), u & 512 && ce(a, a.return);
        break;
      case 12:
        $t(l, a);
        break;
      case 31:
        $t(l, a), u & 4 && lo(l, a);
        break;
      case 13:
        $t(l, a), u & 4 && to(l, a), u & 64 && (l = a.memoizedState, l !== null && (l = l.dehydrated, l !== null && (a = oy.bind(
          null,
          a
        ), jy(l, a))));
        break;
      case 22:
        if (u = a.memoizedState !== null || kt, !u) {
          t = t !== null && t.memoizedState !== null || Nl, e = kt;
          var n = Nl;
          kt = u, (Nl = t) && !n ? Ft(
            l,
            a,
            (a.subtreeFlags & 8772) !== 0
          ) : $t(l, a), kt = e, Nl = n;
        }
        break;
      case 30:
        break;
      default:
        $t(l, a);
    }
  }
  function Iv(l) {
    var t = l.alternate;
    t !== null && (l.alternate = null, Iv(t)), l.child = null, l.deletions = null, l.sibling = null, l.tag === 5 && (t = l.stateNode, t !== null && lc(t)), l.stateNode = null, l.return = null, l.dependencies = null, l.memoizedProps = null, l.memoizedState = null, l.pendingProps = null, l.stateNode = null, l.updateQueue = null;
  }
  var Sl = null, Pl = !1;
  function Wt(l, t, a) {
    for (a = a.child; a !== null; )
      Pv(l, t, a), a = a.sibling;
  }
  function Pv(l, t, a) {
    if (ft && typeof ft.onCommitFiberUnmount == "function")
      try {
        ft.onCommitFiberUnmount(Nu, a);
      } catch {
      }
    switch (a.tag) {
      case 26:
        Nl || Rt(a, t), Wt(
          l,
          t,
          a
        ), a.memoizedState ? a.memoizedState.count-- : a.stateNode && (a = a.stateNode, a.parentNode.removeChild(a));
        break;
      case 27:
        Nl || Rt(a, t);
        var u = Sl, e = Pl;
        _a(a.type) && (Sl = a.stateNode, Pl = !1), Wt(
          l,
          t,
          a
        ), he(a.stateNode), Sl = u, Pl = e;
        break;
      case 5:
        Nl || Rt(a, t);
      case 6:
        if (u = Sl, e = Pl, Sl = null, Wt(
          l,
          t,
          a
        ), Sl = u, Pl = e, Sl !== null)
          if (Pl)
            try {
              (Sl.nodeType === 9 ? Sl.body : Sl.nodeName === "HTML" ? Sl.ownerDocument.body : Sl).removeChild(a.stateNode);
            } catch (n) {
              nl(
                a,
                t,
                n
              );
            }
          else
            try {
              Sl.removeChild(a.stateNode);
            } catch (n) {
              nl(
                a,
                t,
                n
              );
            }
        break;
      case 18:
        Sl !== null && (Pl ? (l = Sl, Jo(
          l.nodeType === 9 ? l.body : l.nodeName === "HTML" ? l.ownerDocument.body : l,
          a.stateNode
        ), Du(l)) : Jo(Sl, a.stateNode));
        break;
      case 4:
        u = Sl, e = Pl, Sl = a.stateNode.containerInfo, Pl = !0, Wt(
          l,
          t,
          a
        ), Sl = u, Pl = e;
        break;
      case 0:
      case 11:
      case 14:
      case 15:
        ya(2, a, t), Nl || ya(4, a, t), Wt(
          l,
          t,
          a
        );
        break;
      case 1:
        Nl || (Rt(a, t), u = a.stateNode, typeof u.componentWillUnmount == "function" && Jv(
          a,
          t,
          u
        )), Wt(
          l,
          t,
          a
        );
        break;
      case 21:
        Wt(
          l,
          t,
          a
        );
        break;
      case 22:
        Nl = (u = Nl) || a.memoizedState !== null, Wt(
          l,
          t,
          a
        ), Nl = u;
        break;
      default:
        Wt(
          l,
          t,
          a
        );
    }
  }
  function lo(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null))) {
      l = l.dehydrated;
      try {
        Du(l);
      } catch (a) {
        nl(t, t.return, a);
      }
    }
  }
  function to(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null && (l = l.dehydrated, l !== null))))
      try {
        Du(l);
      } catch (a) {
        nl(t, t.return, a);
      }
  }
  function uy(l) {
    switch (l.tag) {
      case 31:
      case 13:
      case 19:
        var t = l.stateNode;
        return t === null && (t = l.stateNode = new $v()), t;
      case 22:
        return l = l.stateNode, t = l._retryCache, t === null && (t = l._retryCache = new $v()), t;
      default:
        throw Error(d(435, l.tag));
    }
  }
  function Sn(l, t) {
    var a = uy(l);
    t.forEach(function(u) {
      if (!a.has(u)) {
        a.add(u);
        var e = dy.bind(null, l, u);
        u.then(e, e);
      }
    });
  }
  function lt(l, t) {
    var a = t.deletions;
    if (a !== null)
      for (var u = 0; u < a.length; u++) {
        var e = a[u], n = l, c = t, i = c;
        l: for (; i !== null; ) {
          switch (i.tag) {
            case 27:
              if (_a(i.type)) {
                Sl = i.stateNode, Pl = !1;
                break l;
              }
              break;
            case 5:
              Sl = i.stateNode, Pl = !1;
              break l;
            case 3:
            case 4:
              Sl = i.stateNode.containerInfo, Pl = !0;
              break l;
          }
          i = i.return;
        }
        if (Sl === null) throw Error(d(160));
        Pv(n, c, e), Sl = null, Pl = !1, n = e.alternate, n !== null && (n.return = null), e.return = null;
      }
    if (t.subtreeFlags & 13886)
      for (t = t.child; t !== null; )
        ao(t, l), t = t.sibling;
  }
  var Dt = null;
  function ao(l, t) {
    var a = l.alternate, u = l.flags;
    switch (l.tag) {
      case 0:
      case 11:
      case 14:
      case 15:
        lt(t, l), tt(l), u & 4 && (ya(3, l, l.return), ne(3, l), ya(5, l, l.return));
        break;
      case 1:
        lt(t, l), tt(l), u & 512 && (Nl || a === null || Rt(a, a.return)), u & 64 && kt && (l = l.updateQueue, l !== null && (u = l.callbacks, u !== null && (a = l.shared.hiddenCallbacks, l.shared.hiddenCallbacks = a === null ? u : a.concat(u))));
        break;
      case 26:
        var e = Dt;
        if (lt(t, l), tt(l), u & 512 && (Nl || a === null || Rt(a, a.return)), u & 4) {
          var n = a !== null ? a.memoizedState : null;
          if (u = l.memoizedState, a === null)
            if (u === null)
              if (l.stateNode === null) {
                l: {
                  u = l.type, a = l.memoizedProps, e = e.ownerDocument || e;
                  t: switch (u) {
                    case "title":
                      n = e.getElementsByTagName("title")[0], (!n || n[Ru] || n[Zl] || n.namespaceURI === "http://www.w3.org/2000/svg" || n.hasAttribute("itemprop")) && (n = e.createElement(u), e.head.insertBefore(
                        n,
                        e.querySelector("head > title")
                      )), Jl(n, u, a), n[Zl] = l, ql(n), u = n;
                      break l;
                    case "link":
                      var c = ud(
                        "link",
                        "href",
                        e
                      ).get(u + (a.href || ""));
                      if (c) {
                        for (var i = 0; i < c.length; i++)
                          if (n = c[i], n.getAttribute("href") === (a.href == null || a.href === "" ? null : a.href) && n.getAttribute("rel") === (a.rel == null ? null : a.rel) && n.getAttribute("title") === (a.title == null ? null : a.title) && n.getAttribute("crossorigin") === (a.crossOrigin == null ? null : a.crossOrigin)) {
                            c.splice(i, 1);
                            break t;
                          }
                      }
                      n = e.createElement(u), Jl(n, u, a), e.head.appendChild(n);
                      break;
                    case "meta":
                      if (c = ud(
                        "meta",
                        "content",
                        e
                      ).get(u + (a.content || ""))) {
                        for (i = 0; i < c.length; i++)
                          if (n = c[i], n.getAttribute("content") === (a.content == null ? null : "" + a.content) && n.getAttribute("name") === (a.name == null ? null : a.name) && n.getAttribute("property") === (a.property == null ? null : a.property) && n.getAttribute("http-equiv") === (a.httpEquiv == null ? null : a.httpEquiv) && n.getAttribute("charset") === (a.charSet == null ? null : a.charSet)) {
                            c.splice(i, 1);
                            break t;
                          }
                      }
                      n = e.createElement(u), Jl(n, u, a), e.head.appendChild(n);
                      break;
                    default:
                      throw Error(d(468, u));
                  }
                  n[Zl] = l, ql(n), u = n;
                }
                l.stateNode = u;
              } else
                ed(
                  e,
                  l.type,
                  l.stateNode
                );
            else
              l.stateNode = ad(
                e,
                u,
                l.memoizedProps
              );
          else
            n !== u ? (n === null ? a.stateNode !== null && (a = a.stateNode, a.parentNode.removeChild(a)) : n.count--, u === null ? ed(
              e,
              l.type,
              l.stateNode
            ) : ad(
              e,
              u,
              l.memoizedProps
            )) : u === null && l.stateNode !== null && zi(
              l,
              l.memoizedProps,
              a.memoizedProps
            );
        }
        break;
      case 27:
        lt(t, l), tt(l), u & 512 && (Nl || a === null || Rt(a, a.return)), a !== null && u & 4 && zi(
          l,
          l.memoizedProps,
          a.memoizedProps
        );
        break;
      case 5:
        if (lt(t, l), tt(l), u & 512 && (Nl || a === null || Rt(a, a.return)), l.flags & 32) {
          e = l.stateNode;
          try {
            Fa(e, "");
          } catch (N) {
            nl(l, l.return, N);
          }
        }
        u & 4 && l.stateNode != null && (e = l.memoizedProps, zi(
          l,
          e,
          a !== null ? a.memoizedProps : e
        )), u & 1024 && (Ai = !0);
        break;
      case 6:
        if (lt(t, l), tt(l), u & 4) {
          if (l.stateNode === null)
            throw Error(d(162));
          u = l.memoizedProps, a = l.stateNode;
          try {
            a.nodeValue = u;
          } catch (N) {
            nl(l, l.return, N);
          }
        }
        break;
      case 3:
        if (Rn = null, e = Dt, Dt = jn(t.containerInfo), lt(t, l), Dt = e, tt(l), u & 4 && a !== null && a.memoizedState.isDehydrated)
          try {
            Du(t.containerInfo);
          } catch (N) {
            nl(l, l.return, N);
          }
        Ai && (Ai = !1, uo(l));
        break;
      case 4:
        u = Dt, Dt = jn(
          l.stateNode.containerInfo
        ), lt(t, l), tt(l), Dt = u;
        break;
      case 12:
        lt(t, l), tt(l);
        break;
      case 31:
        lt(t, l), tt(l), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 13:
        lt(t, l), tt(l), l.child.flags & 8192 && l.memoizedState !== null != (a !== null && a.memoizedState !== null) && (bn = it()), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 22:
        e = l.memoizedState !== null;
        var f = a !== null && a.memoizedState !== null, m = kt, r = Nl;
        if (kt = m || e, Nl = r || f, lt(t, l), Nl = r, kt = m, tt(l), u & 8192)
          l: for (t = l.stateNode, t._visibility = e ? t._visibility & -2 : t._visibility | 1, e && (a === null || f || kt || Nl || La(l)), a = null, t = l; ; ) {
            if (t.tag === 5 || t.tag === 26) {
              if (a === null) {
                f = a = t;
                try {
                  if (n = f.stateNode, e)
                    c = n.style, typeof c.setProperty == "function" ? c.setProperty("display", "none", "important") : c.display = "none";
                  else {
                    i = f.stateNode;
                    var z = f.memoizedProps.style, h = z != null && z.hasOwnProperty("display") ? z.display : null;
                    i.style.display = h == null || typeof h == "boolean" ? "" : ("" + h).trim();
                  }
                } catch (N) {
                  nl(f, f.return, N);
                }
              }
            } else if (t.tag === 6) {
              if (a === null) {
                f = t;
                try {
                  f.stateNode.nodeValue = e ? "" : f.memoizedProps;
                } catch (N) {
                  nl(f, f.return, N);
                }
              }
            } else if (t.tag === 18) {
              if (a === null) {
                f = t;
                try {
                  var g = f.stateNode;
                  e ? wo(g, !0) : wo(f.stateNode, !1);
                } catch (N) {
                  nl(f, f.return, N);
                }
              }
            } else if ((t.tag !== 22 && t.tag !== 23 || t.memoizedState === null || t === l) && t.child !== null) {
              t.child.return = t, t = t.child;
              continue;
            }
            if (t === l) break l;
            for (; t.sibling === null; ) {
              if (t.return === null || t.return === l) break l;
              a === t && (a = null), t = t.return;
            }
            a === t && (a = null), t.sibling.return = t.return, t = t.sibling;
          }
        u & 4 && (u = l.updateQueue, u !== null && (a = u.retryQueue, a !== null && (u.retryQueue = null, Sn(l, a))));
        break;
      case 19:
        lt(t, l), tt(l), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 30:
        break;
      case 21:
        break;
      default:
        lt(t, l), tt(l);
    }
  }
  function tt(l) {
    var t = l.flags;
    if (t & 2) {
      try {
        for (var a, u = l.return; u !== null; ) {
          if (kv(u)) {
            a = u;
            break;
          }
          u = u.return;
        }
        if (a == null) throw Error(d(160));
        switch (a.tag) {
          case 27:
            var e = a.stateNode, n = Ei(l);
            gn(l, n, e);
            break;
          case 5:
            var c = a.stateNode;
            a.flags & 32 && (Fa(c, ""), a.flags &= -33);
            var i = Ei(l);
            gn(l, i, c);
            break;
          case 3:
          case 4:
            var f = a.stateNode.containerInfo, m = Ei(l);
            Ti(
              l,
              m,
              f
            );
            break;
          default:
            throw Error(d(161));
        }
      } catch (r) {
        nl(l, l.return, r);
      }
      l.flags &= -3;
    }
    t & 4096 && (l.flags &= -4097);
  }
  function uo(l) {
    if (l.subtreeFlags & 1024)
      for (l = l.child; l !== null; ) {
        var t = l;
        uo(t), t.tag === 5 && t.flags & 1024 && t.stateNode.reset(), l = l.sibling;
      }
  }
  function $t(l, t) {
    if (t.subtreeFlags & 8772)
      for (t = t.child; t !== null; )
        Fv(l, t.alternate, t), t = t.sibling;
  }
  function La(l) {
    for (l = l.child; l !== null; ) {
      var t = l;
      switch (t.tag) {
        case 0:
        case 11:
        case 14:
        case 15:
          ya(4, t, t.return), La(t);
          break;
        case 1:
          Rt(t, t.return);
          var a = t.stateNode;
          typeof a.componentWillUnmount == "function" && Jv(
            t,
            t.return,
            a
          ), La(t);
          break;
        case 27:
          he(t.stateNode);
        case 26:
        case 5:
          Rt(t, t.return), La(t);
          break;
        case 22:
          t.memoizedState === null && La(t);
          break;
        case 30:
          La(t);
          break;
        default:
          La(t);
      }
      l = l.sibling;
    }
  }
  function Ft(l, t, a) {
    for (a = a && (t.subtreeFlags & 8772) !== 0, t = t.child; t !== null; ) {
      var u = t.alternate, e = l, n = t, c = n.flags;
      switch (n.tag) {
        case 0:
        case 11:
        case 15:
          Ft(
            e,
            n,
            a
          ), ne(4, n);
          break;
        case 1:
          if (Ft(
            e,
            n,
            a
          ), u = n, e = u.stateNode, typeof e.componentDidMount == "function")
            try {
              e.componentDidMount();
            } catch (m) {
              nl(u, u.return, m);
            }
          if (u = n, e = u.updateQueue, e !== null) {
            var i = u.stateNode;
            try {
              var f = e.shared.hiddenCallbacks;
              if (f !== null)
                for (e.shared.hiddenCallbacks = null, e = 0; e < f.length; e++)
                  xs(f[e], i);
            } catch (m) {
              nl(u, u.return, m);
            }
          }
          a && c & 64 && Kv(n), ce(n, n.return);
          break;
        case 27:
          Wv(n);
        case 26:
        case 5:
          Ft(
            e,
            n,
            a
          ), a && u === null && c & 4 && wv(n), ce(n, n.return);
          break;
        case 12:
          Ft(
            e,
            n,
            a
          );
          break;
        case 31:
          Ft(
            e,
            n,
            a
          ), a && c & 4 && lo(e, n);
          break;
        case 13:
          Ft(
            e,
            n,
            a
          ), a && c & 4 && to(e, n);
          break;
        case 22:
          n.memoizedState === null && Ft(
            e,
            n,
            a
          ), ce(n, n.return);
          break;
        case 30:
          break;
        default:
          Ft(
            e,
            n,
            a
          );
      }
      t = t.sibling;
    }
  }
  function pi(l, t) {
    var a = null;
    l !== null && l.memoizedState !== null && l.memoizedState.cachePool !== null && (a = l.memoizedState.cachePool.pool), l = null, t.memoizedState !== null && t.memoizedState.cachePool !== null && (l = t.memoizedState.cachePool.pool), l !== a && (l != null && l.refCount++, a != null && Ju(a));
  }
  function Mi(l, t) {
    l = null, t.alternate !== null && (l = t.alternate.memoizedState.cache), t = t.memoizedState.cache, t !== l && (t.refCount++, l != null && Ju(l));
  }
  function Ut(l, t, a, u) {
    if (t.subtreeFlags & 10256)
      for (t = t.child; t !== null; )
        eo(
          l,
          t,
          a,
          u
        ), t = t.sibling;
  }
  function eo(l, t, a, u) {
    var e = t.flags;
    switch (t.tag) {
      case 0:
      case 11:
      case 15:
        Ut(
          l,
          t,
          a,
          u
        ), e & 2048 && ne(9, t);
        break;
      case 1:
        Ut(
          l,
          t,
          a,
          u
        );
        break;
      case 3:
        Ut(
          l,
          t,
          a,
          u
        ), e & 2048 && (l = null, t.alternate !== null && (l = t.alternate.memoizedState.cache), t = t.memoizedState.cache, t !== l && (t.refCount++, l != null && Ju(l)));
        break;
      case 12:
        if (e & 2048) {
          Ut(
            l,
            t,
            a,
            u
          ), l = t.stateNode;
          try {
            var n = t.memoizedProps, c = n.id, i = n.onPostCommit;
            typeof i == "function" && i(
              c,
              t.alternate === null ? "mount" : "update",
              l.passiveEffectDuration,
              -0
            );
          } catch (f) {
            nl(t, t.return, f);
          }
        } else
          Ut(
            l,
            t,
            a,
            u
          );
        break;
      case 31:
        Ut(
          l,
          t,
          a,
          u
        );
        break;
      case 13:
        Ut(
          l,
          t,
          a,
          u
        );
        break;
      case 23:
        break;
      case 22:
        n = t.stateNode, c = t.alternate, t.memoizedState !== null ? n._visibility & 2 ? Ut(
          l,
          t,
          a,
          u
        ) : ie(l, t) : n._visibility & 2 ? Ut(
          l,
          t,
          a,
          u
        ) : (n._visibility |= 2, Su(
          l,
          t,
          a,
          u,
          (t.subtreeFlags & 10256) !== 0 || !1
        )), e & 2048 && pi(c, t);
        break;
      case 24:
        Ut(
          l,
          t,
          a,
          u
        ), e & 2048 && Mi(t.alternate, t);
        break;
      default:
        Ut(
          l,
          t,
          a,
          u
        );
    }
  }
  function Su(l, t, a, u, e) {
    for (e = e && ((t.subtreeFlags & 10256) !== 0 || !1), t = t.child; t !== null; ) {
      var n = l, c = t, i = a, f = u, m = c.flags;
      switch (c.tag) {
        case 0:
        case 11:
        case 15:
          Su(
            n,
            c,
            i,
            f,
            e
          ), ne(8, c);
          break;
        case 23:
          break;
        case 22:
          var r = c.stateNode;
          c.memoizedState !== null ? r._visibility & 2 ? Su(
            n,
            c,
            i,
            f,
            e
          ) : ie(
            n,
            c
          ) : (r._visibility |= 2, Su(
            n,
            c,
            i,
            f,
            e
          )), e && m & 2048 && pi(
            c.alternate,
            c
          );
          break;
        case 24:
          Su(
            n,
            c,
            i,
            f,
            e
          ), e && m & 2048 && Mi(c.alternate, c);
          break;
        default:
          Su(
            n,
            c,
            i,
            f,
            e
          );
      }
      t = t.sibling;
    }
  }
  function ie(l, t) {
    if (t.subtreeFlags & 10256)
      for (t = t.child; t !== null; ) {
        var a = l, u = t, e = u.flags;
        switch (u.tag) {
          case 22:
            ie(a, u), e & 2048 && pi(
              u.alternate,
              u
            );
            break;
          case 24:
            ie(a, u), e & 2048 && Mi(u.alternate, u);
            break;
          default:
            ie(a, u);
        }
        t = t.sibling;
      }
  }
  var fe = 8192;
  function ru(l, t, a) {
    if (l.subtreeFlags & fe)
      for (l = l.child; l !== null; )
        no(
          l,
          t,
          a
        ), l = l.sibling;
  }
  function no(l, t, a) {
    switch (l.tag) {
      case 26:
        ru(
          l,
          t,
          a
        ), l.flags & fe && l.memoizedState !== null && Ly(
          a,
          Dt,
          l.memoizedState,
          l.memoizedProps
        );
        break;
      case 5:
        ru(
          l,
          t,
          a
        );
        break;
      case 3:
      case 4:
        var u = Dt;
        Dt = jn(l.stateNode.containerInfo), ru(
          l,
          t,
          a
        ), Dt = u;
        break;
      case 22:
        l.memoizedState === null && (u = l.alternate, u !== null && u.memoizedState !== null ? (u = fe, fe = 16777216, ru(
          l,
          t,
          a
        ), fe = u) : ru(
          l,
          t,
          a
        ));
        break;
      default:
        ru(
          l,
          t,
          a
        );
    }
  }
  function co(l) {
    var t = l.alternate;
    if (t !== null && (l = t.child, l !== null)) {
      t.child = null;
      do
        t = l.sibling, l.sibling = null, l = t;
      while (l !== null);
    }
  }
  function se(l) {
    var t = l.deletions;
    if ((l.flags & 16) !== 0) {
      if (t !== null)
        for (var a = 0; a < t.length; a++) {
          var u = t[a];
          Bl = u, fo(
            u,
            l
          );
        }
      co(l);
    }
    if (l.subtreeFlags & 10256)
      for (l = l.child; l !== null; )
        io(l), l = l.sibling;
  }
  function io(l) {
    switch (l.tag) {
      case 0:
      case 11:
      case 15:
        se(l), l.flags & 2048 && ya(9, l, l.return);
        break;
      case 3:
        se(l);
        break;
      case 12:
        se(l);
        break;
      case 22:
        var t = l.stateNode;
        l.memoizedState !== null && t._visibility & 2 && (l.return === null || l.return.tag !== 13) ? (t._visibility &= -3, rn(l)) : se(l);
        break;
      default:
        se(l);
    }
  }
  function rn(l) {
    var t = l.deletions;
    if ((l.flags & 16) !== 0) {
      if (t !== null)
        for (var a = 0; a < t.length; a++) {
          var u = t[a];
          Bl = u, fo(
            u,
            l
          );
        }
      co(l);
    }
    for (l = l.child; l !== null; ) {
      switch (t = l, t.tag) {
        case 0:
        case 11:
        case 15:
          ya(8, t, t.return), rn(t);
          break;
        case 22:
          a = t.stateNode, a._visibility & 2 && (a._visibility &= -3, rn(t));
          break;
        default:
          rn(t);
      }
      l = l.sibling;
    }
  }
  function fo(l, t) {
    for (; Bl !== null; ) {
      var a = Bl;
      switch (a.tag) {
        case 0:
        case 11:
        case 15:
          ya(8, a, t);
          break;
        case 23:
        case 22:
          if (a.memoizedState !== null && a.memoizedState.cachePool !== null) {
            var u = a.memoizedState.cachePool.pool;
            u != null && u.refCount++;
          }
          break;
        case 24:
          Ju(a.memoizedState.cache);
      }
      if (u = a.child, u !== null) u.return = a, Bl = u;
      else
        l: for (a = l; Bl !== null; ) {
          u = Bl;
          var e = u.sibling, n = u.return;
          if (Iv(u), u === a) {
            Bl = null;
            break l;
          }
          if (e !== null) {
            e.return = n, Bl = e;
            break l;
          }
          Bl = n;
        }
    }
  }
  var ey = {
    getCacheForType: function(l) {
      var t = Vl(Ol), a = t.data.get(l);
      return a === void 0 && (a = l(), t.data.set(l, a)), a;
    },
    cacheSignal: function() {
      return Vl(Ol).controller.signal;
    }
  }, ny = typeof WeakMap == "function" ? WeakMap : Map, P = 0, dl = null, J = null, k = 0, el = 0, mt = null, ma = !1, bu = !1, Oi = !1, It = 0, zl = 0, ha = 0, Va = 0, Di = 0, ht = 0, _u = 0, ve = null, at = null, Ui = !1, bn = 0, so = 0, _n = 1 / 0, zn = null, ga = null, xl = 0, Sa = null, zu = null, Pt = 0, Ni = 0, ji = null, vo = null, oe = 0, Hi = null;
  function gt() {
    return (P & 2) !== 0 && k !== 0 ? k & -k : S.T !== null ? Yi() : Mf();
  }
  function oo() {
    if (ht === 0)
      if ((k & 536870912) === 0 || $) {
        var l = De;
        De <<= 1, (De & 3932160) === 0 && (De = 262144), ht = l;
      } else ht = 536870912;
    return l = dt.current, l !== null && (l.flags |= 32), ht;
  }
  function ut(l, t, a) {
    (l === dl && (el === 2 || el === 9) || l.cancelPendingCommit !== null) && (Eu(l, 0), ra(
      l,
      k,
      ht,
      !1
    )), Hu(l, a), ((P & 2) === 0 || l !== dl) && (l === dl && ((P & 2) === 0 && (Va |= a), zl === 4 && ra(
      l,
      k,
      ht,
      !1
    )), xt(l));
  }
  function yo(l, t, a) {
    if ((P & 6) !== 0) throw Error(d(327));
    var u = !a && (t & 127) === 0 && (t & l.expiredLanes) === 0 || ju(l, t), e = u ? fy(l, t) : xi(l, t, !0), n = u;
    do {
      if (e === 0) {
        bu && !u && ra(l, t, 0, !1);
        break;
      } else {
        if (a = l.current.alternate, n && !cy(a)) {
          e = xi(l, t, !1), n = !1;
          continue;
        }
        if (e === 2) {
          if (n = t, l.errorRecoveryDisabledLanes & n)
            var c = 0;
          else
            c = l.pendingLanes & -536870913, c = c !== 0 ? c : c & 536870912 ? 536870912 : 0;
          if (c !== 0) {
            t = c;
            l: {
              var i = l;
              e = ve;
              var f = i.current.memoizedState.isDehydrated;
              if (f && (Eu(i, c).flags |= 256), c = xi(
                i,
                c,
                !1
              ), c !== 2) {
                if (Oi && !f) {
                  i.errorRecoveryDisabledLanes |= n, Va |= n, e = 4;
                  break l;
                }
                n = at, at = e, n !== null && (at === null ? at = n : at.push.apply(
                  at,
                  n
                ));
              }
              e = c;
            }
            if (n = !1, e !== 2) continue;
          }
        }
        if (e === 1) {
          Eu(l, 0), ra(l, t, 0, !0);
          break;
        }
        l: {
          switch (u = l, n = e, n) {
            case 0:
            case 1:
              throw Error(d(345));
            case 4:
              if ((t & 4194048) !== t) break;
            case 6:
              ra(
                u,
                t,
                ht,
                !ma
              );
              break l;
            case 2:
              at = null;
              break;
            case 3:
            case 5:
              break;
            default:
              throw Error(d(329));
          }
          if ((t & 62914560) === t && (e = bn + 300 - it(), 10 < e)) {
            if (ra(
              u,
              t,
              ht,
              !ma
            ), Ne(u, 0, !0) !== 0) break l;
            Pt = t, u.timeoutHandle = Vo(
              mo.bind(
                null,
                u,
                a,
                at,
                zn,
                Ui,
                t,
                ht,
                Va,
                _u,
                ma,
                n,
                "Throttled",
                -0,
                0
              ),
              e
            );
            break l;
          }
          mo(
            u,
            a,
            at,
            zn,
            Ui,
            t,
            ht,
            Va,
            _u,
            ma,
            n,
            null,
            -0,
            0
          );
        }
      }
      break;
    } while (!0);
    xt(l);
  }
  function mo(l, t, a, u, e, n, c, i, f, m, r, z, h, g) {
    if (l.timeoutHandle = -1, z = t.subtreeFlags, z & 8192 || (z & 16785408) === 16785408) {
      z = {
        stylesheets: null,
        count: 0,
        imgCount: 0,
        imgBytes: 0,
        suspenseyImages: [],
        waitingForImages: !0,
        waitingForViewTransition: !1,
        unsuspend: Yt
      }, no(
        t,
        n,
        z
      );
      var N = (n & 62914560) === n ? bn - it() : (n & 4194048) === n ? so - it() : 0;
      if (N = Vy(
        z,
        N
      ), N !== null) {
        Pt = n, l.cancelPendingCommit = N(
          Eo.bind(
            null,
            l,
            t,
            n,
            a,
            u,
            e,
            c,
            i,
            f,
            r,
            z,
            null,
            h,
            g
          )
        ), ra(l, n, c, !m);
        return;
      }
    }
    Eo(
      l,
      t,
      n,
      a,
      u,
      e,
      c,
      i,
      f
    );
  }
  function cy(l) {
    for (var t = l; ; ) {
      var a = t.tag;
      if ((a === 0 || a === 11 || a === 15) && t.flags & 16384 && (a = t.updateQueue, a !== null && (a = a.stores, a !== null)))
        for (var u = 0; u < a.length; u++) {
          var e = a[u], n = e.getSnapshot;
          e = e.value;
          try {
            if (!vt(n(), e)) return !1;
          } catch {
            return !1;
          }
        }
      if (a = t.child, t.subtreeFlags & 16384 && a !== null)
        a.return = t, t = a;
      else {
        if (t === l) break;
        for (; t.sibling === null; ) {
          if (t.return === null || t.return === l) return !0;
          t = t.return;
        }
        t.sibling.return = t.return, t = t.sibling;
      }
    }
    return !0;
  }
  function ra(l, t, a, u) {
    t &= ~Di, t &= ~Va, l.suspendedLanes |= t, l.pingedLanes &= ~t, u && (l.warmLanes |= t), u = l.expirationTimes;
    for (var e = t; 0 < e; ) {
      var n = 31 - st(e), c = 1 << n;
      u[n] = -1, e &= ~c;
    }
    a !== 0 && Tf(l, a, t);
  }
  function En() {
    return (P & 6) === 0 ? (de(0), !1) : !0;
  }
  function Ri() {
    if (J !== null) {
      if (el === 0)
        var l = J.return;
      else
        l = J, Zt = qa = null, Wc(l), du = null, ku = 0, l = J;
      for (; l !== null; )
        Vv(l.alternate, l), l = l.return;
      J = null;
    }
  }
  function Eu(l, t) {
    var a = l.timeoutHandle;
    a !== -1 && (l.timeoutHandle = -1, My(a)), a = l.cancelPendingCommit, a !== null && (l.cancelPendingCommit = null, a()), Pt = 0, Ri(), dl = l, J = a = Xt(l.current, null), k = t, el = 0, mt = null, ma = !1, bu = ju(l, t), Oi = !1, _u = ht = Di = Va = ha = zl = 0, at = ve = null, Ui = !1, (t & 8) !== 0 && (t |= t & 32);
    var u = l.entangledLanes;
    if (u !== 0)
      for (l = l.entanglements, u &= t; 0 < u; ) {
        var e = 31 - st(u), n = 1 << e;
        t |= l[e], u &= ~n;
      }
    return It = t, Ze(), a;
  }
  function ho(l, t) {
    X = null, S.H = ae, t === ou || t === $e ? (t = Ns(), el = 3) : t === Cc ? (t = Ns(), el = 4) : el = t === oi ? 8 : t !== null && typeof t == "object" && typeof t.then == "function" ? 6 : 1, mt = t, J === null && (zl = 1, on(
      l,
      _t(t, l.current)
    ));
  }
  function go() {
    var l = dt.current;
    return l === null ? !0 : (k & 4194048) === k ? At === null : (k & 62914560) === k || (k & 536870912) !== 0 ? l === At : !1;
  }
  function So() {
    var l = S.H;
    return S.H = ae, l === null ? ae : l;
  }
  function ro() {
    var l = S.A;
    return S.A = ey, l;
  }
  function Tn() {
    zl = 4, ma || (k & 4194048) !== k && dt.current !== null || (bu = !0), (ha & 134217727) === 0 && (Va & 134217727) === 0 || dl === null || ra(
      dl,
      k,
      ht,
      !1
    );
  }
  function xi(l, t, a) {
    var u = P;
    P |= 2;
    var e = So(), n = ro();
    (dl !== l || k !== t) && (zn = null, Eu(l, t)), t = !1;
    var c = zl;
    l: do
      try {
        if (el !== 0 && J !== null) {
          var i = J, f = mt;
          switch (el) {
            case 8:
              Ri(), c = 6;
              break l;
            case 3:
            case 2:
            case 9:
            case 6:
              dt.current === null && (t = !0);
              var m = el;
              if (el = 0, mt = null, Tu(l, i, f, m), a && bu) {
                c = 0;
                break l;
              }
              break;
            default:
              m = el, el = 0, mt = null, Tu(l, i, f, m);
          }
        }
        iy(), c = zl;
        break;
      } catch (r) {
        ho(l, r);
      }
    while (!0);
    return t && l.shellSuspendCounter++, Zt = qa = null, P = u, S.H = e, S.A = n, J === null && (dl = null, k = 0, Ze()), c;
  }
  function iy() {
    for (; J !== null; ) bo(J);
  }
  function fy(l, t) {
    var a = P;
    P |= 2;
    var u = So(), e = ro();
    dl !== l || k !== t ? (zn = null, _n = it() + 500, Eu(l, t)) : bu = ju(
      l,
      t
    );
    l: do
      try {
        if (el !== 0 && J !== null) {
          t = J;
          var n = mt;
          t: switch (el) {
            case 1:
              el = 0, mt = null, Tu(l, t, n, 1);
              break;
            case 2:
            case 9:
              if (Ds(n)) {
                el = 0, mt = null, _o(t);
                break;
              }
              t = function() {
                el !== 2 && el !== 9 || dl !== l || (el = 7), xt(l);
              }, n.then(t, t);
              break l;
            case 3:
              el = 7;
              break l;
            case 4:
              el = 5;
              break l;
            case 7:
              Ds(n) ? (el = 0, mt = null, _o(t)) : (el = 0, mt = null, Tu(l, t, n, 7));
              break;
            case 5:
              var c = null;
              switch (J.tag) {
                case 26:
                  c = J.memoizedState;
                case 5:
                case 27:
                  var i = J;
                  if (c ? nd(c) : i.stateNode.complete) {
                    el = 0, mt = null;
                    var f = i.sibling;
                    if (f !== null) J = f;
                    else {
                      var m = i.return;
                      m !== null ? (J = m, An(m)) : J = null;
                    }
                    break t;
                  }
              }
              el = 0, mt = null, Tu(l, t, n, 5);
              break;
            case 6:
              el = 0, mt = null, Tu(l, t, n, 6);
              break;
            case 8:
              Ri(), zl = 6;
              break l;
            default:
              throw Error(d(462));
          }
        }
        sy();
        break;
      } catch (r) {
        ho(l, r);
      }
    while (!0);
    return Zt = qa = null, S.H = u, S.A = e, P = a, J !== null ? 0 : (dl = null, k = 0, Ze(), zl);
  }
  function sy() {
    for (; J !== null && !Hd(); )
      bo(J);
  }
  function bo(l) {
    var t = Zv(l.alternate, l, It);
    l.memoizedProps = l.pendingProps, t === null ? An(l) : J = t;
  }
  function _o(l) {
    var t = l, a = t.alternate;
    switch (t.tag) {
      case 15:
      case 0:
        t = Bv(
          a,
          t,
          t.pendingProps,
          t.type,
          void 0,
          k
        );
        break;
      case 11:
        t = Bv(
          a,
          t,
          t.pendingProps,
          t.type.render,
          t.ref,
          k
        );
        break;
      case 5:
        Wc(t);
      default:
        Vv(a, t), t = J = Ss(t, It), t = Zv(a, t, It);
    }
    l.memoizedProps = l.pendingProps, t === null ? An(l) : J = t;
  }
  function Tu(l, t, a, u) {
    Zt = qa = null, Wc(t), du = null, ku = 0;
    var e = t.return;
    try {
      if (F0(
        l,
        e,
        t,
        a,
        k
      )) {
        zl = 1, on(
          l,
          _t(a, l.current)
        ), J = null;
        return;
      }
    } catch (n) {
      if (e !== null) throw J = e, n;
      zl = 1, on(
        l,
        _t(a, l.current)
      ), J = null;
      return;
    }
    t.flags & 32768 ? ($ || u === 1 ? l = !0 : bu || (k & 536870912) !== 0 ? l = !1 : (ma = l = !0, (u === 2 || u === 9 || u === 3 || u === 6) && (u = dt.current, u !== null && u.tag === 13 && (u.flags |= 16384))), zo(t, l)) : An(t);
  }
  function An(l) {
    var t = l;
    do {
      if ((t.flags & 32768) !== 0) {
        zo(
          t,
          ma
        );
        return;
      }
      l = t.return;
      var a = ly(
        t.alternate,
        t,
        It
      );
      if (a !== null) {
        J = a;
        return;
      }
      if (t = t.sibling, t !== null) {
        J = t;
        return;
      }
      J = t = l;
    } while (t !== null);
    zl === 0 && (zl = 5);
  }
  function zo(l, t) {
    do {
      var a = ty(l.alternate, l);
      if (a !== null) {
        a.flags &= 32767, J = a;
        return;
      }
      if (a = l.return, a !== null && (a.flags |= 32768, a.subtreeFlags = 0, a.deletions = null), !t && (l = l.sibling, l !== null)) {
        J = l;
        return;
      }
      J = l = a;
    } while (l !== null);
    zl = 6, J = null;
  }
  function Eo(l, t, a, u, e, n, c, i, f) {
    l.cancelPendingCommit = null;
    do
      pn();
    while (xl !== 0);
    if ((P & 6) !== 0) throw Error(d(327));
    if (t !== null) {
      if (t === l.current) throw Error(d(177));
      if (n = t.lanes | t.childLanes, n |= Ec, Zd(
        l,
        a,
        n,
        c,
        i,
        f
      ), l === dl && (J = dl = null, k = 0), zu = t, Sa = l, Pt = a, Ni = n, ji = e, vo = u, (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? (l.callbackNode = null, l.callbackPriority = 0, yy(Me, function() {
        return Oo(), null;
      })) : (l.callbackNode = null, l.callbackPriority = 0), u = (t.flags & 13878) !== 0, (t.subtreeFlags & 13878) !== 0 || u) {
        u = S.T, S.T = null, e = M.p, M.p = 2, c = P, P |= 4;
        try {
          ay(l, t, a);
        } finally {
          P = c, M.p = e, S.T = u;
        }
      }
      xl = 1, To(), Ao(), po();
    }
  }
  function To() {
    if (xl === 1) {
      xl = 0;
      var l = Sa, t = zu, a = (t.flags & 13878) !== 0;
      if ((t.subtreeFlags & 13878) !== 0 || a) {
        a = S.T, S.T = null;
        var u = M.p;
        M.p = 2;
        var e = P;
        P |= 4;
        try {
          ao(t, l);
          var n = Ji, c = fs(l.containerInfo), i = n.focusedElem, f = n.selectionRange;
          if (c !== i && i && i.ownerDocument && is(
            i.ownerDocument.documentElement,
            i
          )) {
            if (f !== null && Sc(i)) {
              var m = f.start, r = f.end;
              if (r === void 0 && (r = m), "selectionStart" in i)
                i.selectionStart = m, i.selectionEnd = Math.min(
                  r,
                  i.value.length
                );
              else {
                var z = i.ownerDocument || document, h = z && z.defaultView || window;
                if (h.getSelection) {
                  var g = h.getSelection(), N = i.textContent.length, B = Math.min(f.start, N), vl = f.end === void 0 ? B : Math.min(f.end, N);
                  !g.extend && B > vl && (c = vl, vl = B, B = c);
                  var o = cs(
                    i,
                    B
                  ), s = cs(
                    i,
                    vl
                  );
                  if (o && s && (g.rangeCount !== 1 || g.anchorNode !== o.node || g.anchorOffset !== o.offset || g.focusNode !== s.node || g.focusOffset !== s.offset)) {
                    var y = z.createRange();
                    y.setStart(o.node, o.offset), g.removeAllRanges(), B > vl ? (g.addRange(y), g.extend(s.node, s.offset)) : (y.setEnd(s.node, s.offset), g.addRange(y));
                  }
                }
              }
            }
            for (z = [], g = i; g = g.parentNode; )
              g.nodeType === 1 && z.push({
                element: g,
                left: g.scrollLeft,
                top: g.scrollTop
              });
            for (typeof i.focus == "function" && i.focus(), i = 0; i < z.length; i++) {
              var b = z[i];
              b.element.scrollLeft = b.left, b.element.scrollTop = b.top;
            }
          }
          Cn = !!Ki, Ji = Ki = null;
        } finally {
          P = e, M.p = u, S.T = a;
        }
      }
      l.current = t, xl = 2;
    }
  }
  function Ao() {
    if (xl === 2) {
      xl = 0;
      var l = Sa, t = zu, a = (t.flags & 8772) !== 0;
      if ((t.subtreeFlags & 8772) !== 0 || a) {
        a = S.T, S.T = null;
        var u = M.p;
        M.p = 2;
        var e = P;
        P |= 4;
        try {
          Fv(l, t.alternate, t);
        } finally {
          P = e, M.p = u, S.T = a;
        }
      }
      xl = 3;
    }
  }
  function po() {
    if (xl === 4 || xl === 3) {
      xl = 0, Rd();
      var l = Sa, t = zu, a = Pt, u = vo;
      (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? xl = 5 : (xl = 0, zu = Sa = null, Mo(l, l.pendingLanes));
      var e = l.pendingLanes;
      if (e === 0 && (ga = null), In(a), t = t.stateNode, ft && typeof ft.onCommitFiberRoot == "function")
        try {
          ft.onCommitFiberRoot(
            Nu,
            t,
            void 0,
            (t.current.flags & 128) === 128
          );
        } catch {
        }
      if (u !== null) {
        t = S.T, e = M.p, M.p = 2, S.T = null;
        try {
          for (var n = l.onRecoverableError, c = 0; c < u.length; c++) {
            var i = u[c];
            n(i.value, {
              componentStack: i.stack
            });
          }
        } finally {
          S.T = t, M.p = e;
        }
      }
      (Pt & 3) !== 0 && pn(), xt(l), e = l.pendingLanes, (a & 261930) !== 0 && (e & 42) !== 0 ? l === Hi ? oe++ : (oe = 0, Hi = l) : oe = 0, de(0);
    }
  }
  function Mo(l, t) {
    (l.pooledCacheLanes &= t) === 0 && (t = l.pooledCache, t != null && (l.pooledCache = null, Ju(t)));
  }
  function pn() {
    return To(), Ao(), po(), Oo();
  }
  function Oo() {
    if (xl !== 5) return !1;
    var l = Sa, t = Ni;
    Ni = 0;
    var a = In(Pt), u = S.T, e = M.p;
    try {
      M.p = 32 > a ? 32 : a, S.T = null, a = ji, ji = null;
      var n = Sa, c = Pt;
      if (xl = 0, zu = Sa = null, Pt = 0, (P & 6) !== 0) throw Error(d(331));
      var i = P;
      if (P |= 4, io(n.current), eo(
        n,
        n.current,
        c,
        a
      ), P = i, de(0, !1), ft && typeof ft.onPostCommitFiberRoot == "function")
        try {
          ft.onPostCommitFiberRoot(Nu, n);
        } catch {
        }
      return !0;
    } finally {
      M.p = e, S.T = u, Mo(l, t);
    }
  }
  function Do(l, t, a) {
    t = _t(a, t), t = vi(l.stateNode, t, 2), l = va(l, t, 2), l !== null && (Hu(l, 2), xt(l));
  }
  function nl(l, t, a) {
    if (l.tag === 3)
      Do(l, l, a);
    else
      for (; t !== null; ) {
        if (t.tag === 3) {
          Do(
            t,
            l,
            a
          );
          break;
        } else if (t.tag === 1) {
          var u = t.stateNode;
          if (typeof t.type.getDerivedStateFromError == "function" || typeof u.componentDidCatch == "function" && (ga === null || !ga.has(u))) {
            l = _t(a, l), a = Dv(2), u = va(t, a, 2), u !== null && (Uv(
              a,
              u,
              t,
              l
            ), Hu(u, 2), xt(u));
            break;
          }
        }
        t = t.return;
      }
  }
  function qi(l, t, a) {
    var u = l.pingCache;
    if (u === null) {
      u = l.pingCache = new ny();
      var e = /* @__PURE__ */ new Set();
      u.set(t, e);
    } else
      e = u.get(t), e === void 0 && (e = /* @__PURE__ */ new Set(), u.set(t, e));
    e.has(a) || (Oi = !0, e.add(a), l = vy.bind(null, l, t, a), t.then(l, l));
  }
  function vy(l, t, a) {
    var u = l.pingCache;
    u !== null && u.delete(t), l.pingedLanes |= l.suspendedLanes & a, l.warmLanes &= ~a, dl === l && (k & a) === a && (zl === 4 || zl === 3 && (k & 62914560) === k && 300 > it() - bn ? (P & 2) === 0 && Eu(l, 0) : Di |= a, _u === k && (_u = 0)), xt(l);
  }
  function Uo(l, t) {
    t === 0 && (t = Ef()), l = Ha(l, t), l !== null && (Hu(l, t), xt(l));
  }
  function oy(l) {
    var t = l.memoizedState, a = 0;
    t !== null && (a = t.retryLane), Uo(l, a);
  }
  function dy(l, t) {
    var a = 0;
    switch (l.tag) {
      case 31:
      case 13:
        var u = l.stateNode, e = l.memoizedState;
        e !== null && (a = e.retryLane);
        break;
      case 19:
        u = l.stateNode;
        break;
      case 22:
        u = l.stateNode._retryCache;
        break;
      default:
        throw Error(d(314));
    }
    u !== null && u.delete(t), Uo(l, a);
  }
  function yy(l, t) {
    return kn(l, t);
  }
  var Mn = null, Au = null, Bi = !1, On = !1, Ci = !1, ba = 0;
  function xt(l) {
    l !== Au && l.next === null && (Au === null ? Mn = Au = l : Au = Au.next = l), On = !0, Bi || (Bi = !0, hy());
  }
  function de(l, t) {
    if (!Ci && On) {
      Ci = !0;
      do
        for (var a = !1, u = Mn; u !== null; ) {
          if (l !== 0) {
            var e = u.pendingLanes;
            if (e === 0) var n = 0;
            else {
              var c = u.suspendedLanes, i = u.pingedLanes;
              n = (1 << 31 - st(42 | l) + 1) - 1, n &= e & ~(c & ~i), n = n & 201326741 ? n & 201326741 | 1 : n ? n | 2 : 0;
            }
            n !== 0 && (a = !0, Ro(u, n));
          } else
            n = k, n = Ne(
              u,
              u === dl ? n : 0,
              u.cancelPendingCommit !== null || u.timeoutHandle !== -1
            ), (n & 3) === 0 || ju(u, n) || (a = !0, Ro(u, n));
          u = u.next;
        }
      while (a);
      Ci = !1;
    }
  }
  function my() {
    No();
  }
  function No() {
    On = Bi = !1;
    var l = 0;
    ba !== 0 && py() && (l = ba);
    for (var t = it(), a = null, u = Mn; u !== null; ) {
      var e = u.next, n = jo(u, t);
      n === 0 ? (u.next = null, a === null ? Mn = e : a.next = e, e === null && (Au = a)) : (a = u, (l !== 0 || (n & 3) !== 0) && (On = !0)), u = e;
    }
    xl !== 0 && xl !== 5 || de(l), ba !== 0 && (ba = 0);
  }
  function jo(l, t) {
    for (var a = l.suspendedLanes, u = l.pingedLanes, e = l.expirationTimes, n = l.pendingLanes & -62914561; 0 < n; ) {
      var c = 31 - st(n), i = 1 << c, f = e[c];
      f === -1 ? ((i & a) === 0 || (i & u) !== 0) && (e[c] = Qd(i, t)) : f <= t && (l.expiredLanes |= i), n &= ~i;
    }
    if (t = dl, a = k, a = Ne(
      l,
      l === t ? a : 0,
      l.cancelPendingCommit !== null || l.timeoutHandle !== -1
    ), u = l.callbackNode, a === 0 || l === t && (el === 2 || el === 9) || l.cancelPendingCommit !== null)
      return u !== null && u !== null && Wn(u), l.callbackNode = null, l.callbackPriority = 0;
    if ((a & 3) === 0 || ju(l, a)) {
      if (t = a & -a, t === l.callbackPriority) return t;
      switch (u !== null && Wn(u), In(a)) {
        case 2:
        case 8:
          a = _f;
          break;
        case 32:
          a = Me;
          break;
        case 268435456:
          a = zf;
          break;
        default:
          a = Me;
      }
      return u = Ho.bind(null, l), a = kn(a, u), l.callbackPriority = t, l.callbackNode = a, t;
    }
    return u !== null && u !== null && Wn(u), l.callbackPriority = 2, l.callbackNode = null, 2;
  }
  function Ho(l, t) {
    if (xl !== 0 && xl !== 5)
      return l.callbackNode = null, l.callbackPriority = 0, null;
    var a = l.callbackNode;
    if (pn() && l.callbackNode !== a)
      return null;
    var u = k;
    return u = Ne(
      l,
      l === dl ? u : 0,
      l.cancelPendingCommit !== null || l.timeoutHandle !== -1
    ), u === 0 ? null : (yo(l, u, t), jo(l, it()), l.callbackNode != null && l.callbackNode === a ? Ho.bind(null, l) : null);
  }
  function Ro(l, t) {
    if (pn()) return null;
    yo(l, t, !0);
  }
  function hy() {
    Oy(function() {
      (P & 6) !== 0 ? kn(
        bf,
        my
      ) : No();
    });
  }
  function Yi() {
    if (ba === 0) {
      var l = su;
      l === 0 && (l = Oe, Oe <<= 1, (Oe & 261888) === 0 && (Oe = 256)), ba = l;
    }
    return ba;
  }
  function xo(l) {
    return l == null || typeof l == "symbol" || typeof l == "boolean" ? null : typeof l == "function" ? l : xe("" + l);
  }
  function qo(l, t) {
    var a = t.ownerDocument.createElement("input");
    return a.name = t.name, a.value = t.value, l.id && a.setAttribute("form", l.id), t.parentNode.insertBefore(a, t), l = new FormData(l), a.parentNode.removeChild(a), l;
  }
  function gy(l, t, a, u, e) {
    if (t === "submit" && a && a.stateNode === e) {
      var n = xo(
        (e[Fl] || null).action
      ), c = u.submitter;
      c && (t = (t = c[Fl] || null) ? xo(t.formAction) : c.getAttribute("formAction"), t !== null && (n = t, c = null));
      var i = new Ye(
        "action",
        "action",
        null,
        u,
        e
      );
      l.push({
        event: i,
        listeners: [
          {
            instance: null,
            listener: function() {
              if (u.defaultPrevented) {
                if (ba !== 0) {
                  var f = c ? qo(e, c) : new FormData(e);
                  ei(
                    a,
                    {
                      pending: !0,
                      data: f,
                      method: e.method,
                      action: n
                    },
                    null,
                    f
                  );
                }
              } else
                typeof n == "function" && (i.preventDefault(), f = c ? qo(e, c) : new FormData(e), ei(
                  a,
                  {
                    pending: !0,
                    data: f,
                    method: e.method,
                    action: n
                  },
                  n,
                  f
                ));
            },
            currentTarget: e
          }
        ]
      });
    }
  }
  for (var Gi = 0; Gi < zc.length; Gi++) {
    var Xi = zc[Gi], Sy = Xi.toLowerCase(), ry = Xi[0].toUpperCase() + Xi.slice(1);
    Ot(
      Sy,
      "on" + ry
    );
  }
  Ot(os, "onAnimationEnd"), Ot(ds, "onAnimationIteration"), Ot(ys, "onAnimationStart"), Ot("dblclick", "onDoubleClick"), Ot("focusin", "onFocus"), Ot("focusout", "onBlur"), Ot(x0, "onTransitionRun"), Ot(q0, "onTransitionStart"), Ot(B0, "onTransitionCancel"), Ot(ms, "onTransitionEnd"), Wa("onMouseEnter", ["mouseout", "mouseover"]), Wa("onMouseLeave", ["mouseout", "mouseover"]), Wa("onPointerEnter", ["pointerout", "pointerover"]), Wa("onPointerLeave", ["pointerout", "pointerover"]), Da(
    "onChange",
    "change click focusin focusout input keydown keyup selectionchange".split(" ")
  ), Da(
    "onSelect",
    "focusout contextmenu dragend focusin keydown keyup mousedown mouseup selectionchange".split(
      " "
    )
  ), Da("onBeforeInput", [
    "compositionend",
    "keypress",
    "textInput",
    "paste"
  ]), Da(
    "onCompositionEnd",
    "compositionend focusout keydown keypress keyup mousedown".split(" ")
  ), Da(
    "onCompositionStart",
    "compositionstart focusout keydown keypress keyup mousedown".split(" ")
  ), Da(
    "onCompositionUpdate",
    "compositionupdate focusout keydown keypress keyup mousedown".split(" ")
  );
  var ye = "abort canplay canplaythrough durationchange emptied encrypted ended error loadeddata loadedmetadata loadstart pause play playing progress ratechange resize seeked seeking stalled suspend timeupdate volumechange waiting".split(
    " "
  ), by = new Set(
    "beforetoggle cancel close invalid load scroll scrollend toggle".split(" ").concat(ye)
  );
  function Bo(l, t) {
    t = (t & 4) !== 0;
    for (var a = 0; a < l.length; a++) {
      var u = l[a], e = u.event;
      u = u.listeners;
      l: {
        var n = void 0;
        if (t)
          for (var c = u.length - 1; 0 <= c; c--) {
            var i = u[c], f = i.instance, m = i.currentTarget;
            if (i = i.listener, f !== n && e.isPropagationStopped())
              break l;
            n = i, e.currentTarget = m;
            try {
              n(e);
            } catch (r) {
              Qe(r);
            }
            e.currentTarget = null, n = f;
          }
        else
          for (c = 0; c < u.length; c++) {
            if (i = u[c], f = i.instance, m = i.currentTarget, i = i.listener, f !== n && e.isPropagationStopped())
              break l;
            n = i, e.currentTarget = m;
            try {
              n(e);
            } catch (r) {
              Qe(r);
            }
            e.currentTarget = null, n = f;
          }
      }
    }
  }
  function w(l, t) {
    var a = t[Pn];
    a === void 0 && (a = t[Pn] = /* @__PURE__ */ new Set());
    var u = l + "__bubble";
    a.has(u) || (Co(t, l, 2, !1), a.add(u));
  }
  function Qi(l, t, a) {
    var u = 0;
    t && (u |= 4), Co(
      a,
      l,
      u,
      t
    );
  }
  var Dn = "_reactListening" + Math.random().toString(36).slice(2);
  function Zi(l) {
    if (!l[Dn]) {
      l[Dn] = !0, Uf.forEach(function(a) {
        a !== "selectionchange" && (by.has(a) || Qi(a, !1, l), Qi(a, !0, l));
      });
      var t = l.nodeType === 9 ? l : l.ownerDocument;
      t === null || t[Dn] || (t[Dn] = !0, Qi("selectionchange", !1, t));
    }
  }
  function Co(l, t, a, u) {
    switch (dd(t)) {
      case 2:
        var e = wy;
        break;
      case 8:
        e = ky;
        break;
      default:
        e = uf;
    }
    a = e.bind(
      null,
      t,
      a,
      l
    ), e = void 0, !fc || t !== "touchstart" && t !== "touchmove" && t !== "wheel" || (e = !0), u ? e !== void 0 ? l.addEventListener(t, a, {
      capture: !0,
      passive: e
    }) : l.addEventListener(t, a, !0) : e !== void 0 ? l.addEventListener(t, a, {
      passive: e
    }) : l.addEventListener(t, a, !1);
  }
  function Li(l, t, a, u, e) {
    var n = u;
    if ((t & 1) === 0 && (t & 2) === 0 && u !== null)
      l: for (; ; ) {
        if (u === null) return;
        var c = u.tag;
        if (c === 3 || c === 4) {
          var i = u.stateNode.containerInfo;
          if (i === e) break;
          if (c === 4)
            for (c = u.return; c !== null; ) {
              var f = c.tag;
              if ((f === 3 || f === 4) && c.stateNode.containerInfo === e)
                return;
              c = c.return;
            }
          for (; i !== null; ) {
            if (c = Ja(i), c === null) return;
            if (f = c.tag, f === 5 || f === 6 || f === 26 || f === 27) {
              u = n = c;
              continue l;
            }
            i = i.parentNode;
          }
        }
        u = u.return;
      }
    Qf(function() {
      var m = n, r = cc(a), z = [];
      l: {
        var h = hs.get(l);
        if (h !== void 0) {
          var g = Ye, N = l;
          switch (l) {
            case "keypress":
              if (Be(a) === 0) break l;
            case "keydown":
            case "keyup":
              g = d0;
              break;
            case "focusin":
              N = "focus", g = dc;
              break;
            case "focusout":
              N = "blur", g = dc;
              break;
            case "beforeblur":
            case "afterblur":
              g = dc;
              break;
            case "click":
              if (a.button === 2) break l;
            case "auxclick":
            case "dblclick":
            case "mousedown":
            case "mousemove":
            case "mouseup":
            case "mouseout":
            case "mouseover":
            case "contextmenu":
              g = Vf;
              break;
            case "drag":
            case "dragend":
            case "dragenter":
            case "dragexit":
            case "dragleave":
            case "dragover":
            case "dragstart":
            case "drop":
              g = l0;
              break;
            case "touchcancel":
            case "touchend":
            case "touchmove":
            case "touchstart":
              g = h0;
              break;
            case os:
            case ds:
            case ys:
              g = u0;
              break;
            case ms:
              g = S0;
              break;
            case "scroll":
            case "scrollend":
              g = Id;
              break;
            case "wheel":
              g = b0;
              break;
            case "copy":
            case "cut":
            case "paste":
              g = n0;
              break;
            case "gotpointercapture":
            case "lostpointercapture":
            case "pointercancel":
            case "pointerdown":
            case "pointermove":
            case "pointerout":
            case "pointerover":
            case "pointerup":
              g = Jf;
              break;
            case "toggle":
            case "beforetoggle":
              g = z0;
          }
          var B = (t & 4) !== 0, vl = !B && (l === "scroll" || l === "scrollend"), o = B ? h !== null ? h + "Capture" : null : h;
          B = [];
          for (var s = m, y; s !== null; ) {
            var b = s;
            if (y = b.stateNode, b = b.tag, b !== 5 && b !== 26 && b !== 27 || y === null || o === null || (b = qu(s, o), b != null && B.push(
              me(s, b, y)
            )), vl) break;
            s = s.return;
          }
          0 < B.length && (h = new g(
            h,
            N,
            null,
            a,
            r
          ), z.push({ event: h, listeners: B }));
        }
      }
      if ((t & 7) === 0) {
        l: {
          if (h = l === "mouseover" || l === "pointerover", g = l === "mouseout" || l === "pointerout", h && a !== nc && (N = a.relatedTarget || a.fromElement) && (Ja(N) || N[Ka]))
            break l;
          if ((g || h) && (h = r.window === r ? r : (h = r.ownerDocument) ? h.defaultView || h.parentWindow : window, g ? (N = a.relatedTarget || a.toElement, g = m, N = N ? Ja(N) : null, N !== null && (vl = yl(N), B = N.tag, N !== vl || B !== 5 && B !== 27 && B !== 6) && (N = null)) : (g = null, N = m), g !== N)) {
            if (B = Vf, b = "onMouseLeave", o = "onMouseEnter", s = "mouse", (l === "pointerout" || l === "pointerover") && (B = Jf, b = "onPointerLeave", o = "onPointerEnter", s = "pointer"), vl = g == null ? h : xu(g), y = N == null ? h : xu(N), h = new B(
              b,
              s + "leave",
              g,
              a,
              r
            ), h.target = vl, h.relatedTarget = y, b = null, Ja(r) === m && (B = new B(
              o,
              s + "enter",
              N,
              a,
              r
            ), B.target = y, B.relatedTarget = vl, b = B), vl = b, g && N)
              t: {
                for (B = _y, o = g, s = N, y = 0, b = o; b; b = B(b))
                  y++;
                b = 0;
                for (var q = s; q; q = B(q))
                  b++;
                for (; 0 < y - b; )
                  o = B(o), y--;
                for (; 0 < b - y; )
                  s = B(s), b--;
                for (; y--; ) {
                  if (o === s || s !== null && o === s.alternate) {
                    B = o;
                    break t;
                  }
                  o = B(o), s = B(s);
                }
                B = null;
              }
            else B = null;
            g !== null && Yo(
              z,
              h,
              g,
              B,
              !1
            ), N !== null && vl !== null && Yo(
              z,
              vl,
              N,
              B,
              !0
            );
          }
        }
        l: {
          if (h = m ? xu(m) : window, g = h.nodeName && h.nodeName.toLowerCase(), g === "select" || g === "input" && h.type === "file")
            var F = ls;
          else if (If(h))
            if (ts)
              F = j0;
            else {
              F = U0;
              var R = D0;
            }
          else
            g = h.nodeName, !g || g.toLowerCase() !== "input" || h.type !== "checkbox" && h.type !== "radio" ? m && ec(m.elementType) && (F = ls) : F = N0;
          if (F && (F = F(l, m))) {
            Pf(
              z,
              F,
              a,
              r
            );
            break l;
          }
          R && R(l, h, m), l === "focusout" && m && h.type === "number" && m.memoizedProps.value != null && uc(h, "number", h.value);
        }
        switch (R = m ? xu(m) : window, l) {
          case "focusin":
            (If(R) || R.contentEditable === "true") && (tu = R, rc = m, Lu = null);
            break;
          case "focusout":
            Lu = rc = tu = null;
            break;
          case "mousedown":
            bc = !0;
            break;
          case "contextmenu":
          case "mouseup":
          case "dragend":
            bc = !1, ss(z, a, r);
            break;
          case "selectionchange":
            if (R0) break;
          case "keydown":
          case "keyup":
            ss(z, a, r);
        }
        var Q;
        if (mc)
          l: {
            switch (l) {
              case "compositionstart":
                var W = "onCompositionStart";
                break l;
              case "compositionend":
                W = "onCompositionEnd";
                break l;
              case "compositionupdate":
                W = "onCompositionUpdate";
                break l;
            }
            W = void 0;
          }
        else
          lu ? $f(l, a) && (W = "onCompositionEnd") : l === "keydown" && a.keyCode === 229 && (W = "onCompositionStart");
        W && (wf && a.locale !== "ko" && (lu || W !== "onCompositionStart" ? W === "onCompositionEnd" && lu && (Q = Zf()) : (ua = r, sc = "value" in ua ? ua.value : ua.textContent, lu = !0)), R = Un(m, W), 0 < R.length && (W = new Kf(
          W,
          l,
          null,
          a,
          r
        ), z.push({ event: W, listeners: R }), Q ? W.data = Q : (Q = Ff(a), Q !== null && (W.data = Q)))), (Q = T0 ? A0(l, a) : p0(l, a)) && (W = Un(m, "onBeforeInput"), 0 < W.length && (R = new Kf(
          "onBeforeInput",
          "beforeinput",
          null,
          a,
          r
        ), z.push({
          event: R,
          listeners: W
        }), R.data = Q)), gy(
          z,
          l,
          m,
          a,
          r
        );
      }
      Bo(z, t);
    });
  }
  function me(l, t, a) {
    return {
      instance: l,
      listener: t,
      currentTarget: a
    };
  }
  function Un(l, t) {
    for (var a = t + "Capture", u = []; l !== null; ) {
      var e = l, n = e.stateNode;
      if (e = e.tag, e !== 5 && e !== 26 && e !== 27 || n === null || (e = qu(l, a), e != null && u.unshift(
        me(l, e, n)
      ), e = qu(l, t), e != null && u.push(
        me(l, e, n)
      )), l.tag === 3) return u;
      l = l.return;
    }
    return [];
  }
  function _y(l) {
    if (l === null) return null;
    do
      l = l.return;
    while (l && l.tag !== 5 && l.tag !== 27);
    return l || null;
  }
  function Yo(l, t, a, u, e) {
    for (var n = t._reactName, c = []; a !== null && a !== u; ) {
      var i = a, f = i.alternate, m = i.stateNode;
      if (i = i.tag, f !== null && f === u) break;
      i !== 5 && i !== 26 && i !== 27 || m === null || (f = m, e ? (m = qu(a, n), m != null && c.unshift(
        me(a, m, f)
      )) : e || (m = qu(a, n), m != null && c.push(
        me(a, m, f)
      ))), a = a.return;
    }
    c.length !== 0 && l.push({ event: t, listeners: c });
  }
  var zy = /\r\n?/g, Ey = /\u0000|\uFFFD/g;
  function Go(l) {
    return (typeof l == "string" ? l : "" + l).replace(zy, `
`).replace(Ey, "");
  }
  function Xo(l, t) {
    return t = Go(t), Go(l) === t;
  }
  function sl(l, t, a, u, e, n) {
    switch (a) {
      case "children":
        typeof u == "string" ? t === "body" || t === "textarea" && u === "" || Fa(l, u) : (typeof u == "number" || typeof u == "bigint") && t !== "body" && Fa(l, "" + u);
        break;
      case "className":
        He(l, "class", u);
        break;
      case "tabIndex":
        He(l, "tabindex", u);
        break;
      case "dir":
      case "role":
      case "viewBox":
      case "width":
      case "height":
        He(l, a, u);
        break;
      case "style":
        Gf(l, u, n);
        break;
      case "data":
        if (t !== "object") {
          He(l, "data", u);
          break;
        }
      case "src":
      case "href":
        if (u === "" && (t !== "a" || a !== "href")) {
          l.removeAttribute(a);
          break;
        }
        if (u == null || typeof u == "function" || typeof u == "symbol" || typeof u == "boolean") {
          l.removeAttribute(a);
          break;
        }
        u = xe("" + u), l.setAttribute(a, u);
        break;
      case "action":
      case "formAction":
        if (typeof u == "function") {
          l.setAttribute(
            a,
            "javascript:throw new Error('A React form was unexpectedly submitted. If you called form.submit() manually, consider using form.requestSubmit() instead. If you\\'re trying to use event.stopPropagation() in a submit event handler, consider also calling event.preventDefault().')"
          );
          break;
        } else
          typeof n == "function" && (a === "formAction" ? (t !== "input" && sl(l, t, "name", e.name, e, null), sl(
            l,
            t,
            "formEncType",
            e.formEncType,
            e,
            null
          ), sl(
            l,
            t,
            "formMethod",
            e.formMethod,
            e,
            null
          ), sl(
            l,
            t,
            "formTarget",
            e.formTarget,
            e,
            null
          )) : (sl(l, t, "encType", e.encType, e, null), sl(l, t, "method", e.method, e, null), sl(l, t, "target", e.target, e, null)));
        if (u == null || typeof u == "symbol" || typeof u == "boolean") {
          l.removeAttribute(a);
          break;
        }
        u = xe("" + u), l.setAttribute(a, u);
        break;
      case "onClick":
        u != null && (l.onclick = Yt);
        break;
      case "onScroll":
        u != null && w("scroll", l);
        break;
      case "onScrollEnd":
        u != null && w("scrollend", l);
        break;
      case "dangerouslySetInnerHTML":
        if (u != null) {
          if (typeof u != "object" || !("__html" in u))
            throw Error(d(61));
          if (a = u.__html, a != null) {
            if (e.children != null) throw Error(d(60));
            l.innerHTML = a;
          }
        }
        break;
      case "multiple":
        l.multiple = u && typeof u != "function" && typeof u != "symbol";
        break;
      case "muted":
        l.muted = u && typeof u != "function" && typeof u != "symbol";
        break;
      case "suppressContentEditableWarning":
      case "suppressHydrationWarning":
      case "defaultValue":
      case "defaultChecked":
      case "innerHTML":
      case "ref":
        break;
      case "autoFocus":
        break;
      case "xlinkHref":
        if (u == null || typeof u == "function" || typeof u == "boolean" || typeof u == "symbol") {
          l.removeAttribute("xlink:href");
          break;
        }
        a = xe("" + u), l.setAttributeNS(
          "http://www.w3.org/1999/xlink",
          "xlink:href",
          a
        );
        break;
      case "contentEditable":
      case "spellCheck":
      case "draggable":
      case "value":
      case "autoReverse":
      case "externalResourcesRequired":
      case "focusable":
      case "preserveAlpha":
        u != null && typeof u != "function" && typeof u != "symbol" ? l.setAttribute(a, "" + u) : l.removeAttribute(a);
        break;
      case "inert":
      case "allowFullScreen":
      case "async":
      case "autoPlay":
      case "controls":
      case "default":
      case "defer":
      case "disabled":
      case "disablePictureInPicture":
      case "disableRemotePlayback":
      case "formNoValidate":
      case "hidden":
      case "loop":
      case "noModule":
      case "noValidate":
      case "open":
      case "playsInline":
      case "readOnly":
      case "required":
      case "reversed":
      case "scoped":
      case "seamless":
      case "itemScope":
        u && typeof u != "function" && typeof u != "symbol" ? l.setAttribute(a, "") : l.removeAttribute(a);
        break;
      case "capture":
      case "download":
        u === !0 ? l.setAttribute(a, "") : u !== !1 && u != null && typeof u != "function" && typeof u != "symbol" ? l.setAttribute(a, u) : l.removeAttribute(a);
        break;
      case "cols":
      case "rows":
      case "size":
      case "span":
        u != null && typeof u != "function" && typeof u != "symbol" && !isNaN(u) && 1 <= u ? l.setAttribute(a, u) : l.removeAttribute(a);
        break;
      case "rowSpan":
      case "start":
        u == null || typeof u == "function" || typeof u == "symbol" || isNaN(u) ? l.removeAttribute(a) : l.setAttribute(a, u);
        break;
      case "popover":
        w("beforetoggle", l), w("toggle", l), je(l, "popover", u);
        break;
      case "xlinkActuate":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:actuate",
          u
        );
        break;
      case "xlinkArcrole":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:arcrole",
          u
        );
        break;
      case "xlinkRole":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:role",
          u
        );
        break;
      case "xlinkShow":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:show",
          u
        );
        break;
      case "xlinkTitle":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:title",
          u
        );
        break;
      case "xlinkType":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:type",
          u
        );
        break;
      case "xmlBase":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:base",
          u
        );
        break;
      case "xmlLang":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:lang",
          u
        );
        break;
      case "xmlSpace":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:space",
          u
        );
        break;
      case "is":
        je(l, "is", u);
        break;
      case "innerText":
      case "textContent":
        break;
      default:
        (!(2 < a.length) || a[0] !== "o" && a[0] !== "O" || a[1] !== "n" && a[1] !== "N") && (a = $d.get(a) || a, je(l, a, u));
    }
  }
  function Vi(l, t, a, u, e, n) {
    switch (a) {
      case "style":
        Gf(l, u, n);
        break;
      case "dangerouslySetInnerHTML":
        if (u != null) {
          if (typeof u != "object" || !("__html" in u))
            throw Error(d(61));
          if (a = u.__html, a != null) {
            if (e.children != null) throw Error(d(60));
            l.innerHTML = a;
          }
        }
        break;
      case "children":
        typeof u == "string" ? Fa(l, u) : (typeof u == "number" || typeof u == "bigint") && Fa(l, "" + u);
        break;
      case "onScroll":
        u != null && w("scroll", l);
        break;
      case "onScrollEnd":
        u != null && w("scrollend", l);
        break;
      case "onClick":
        u != null && (l.onclick = Yt);
        break;
      case "suppressContentEditableWarning":
      case "suppressHydrationWarning":
      case "innerHTML":
      case "ref":
        break;
      case "innerText":
      case "textContent":
        break;
      default:
        if (!Nf.hasOwnProperty(a))
          l: {
            if (a[0] === "o" && a[1] === "n" && (e = a.endsWith("Capture"), t = a.slice(2, e ? a.length - 7 : void 0), n = l[Fl] || null, n = n != null ? n[a] : null, typeof n == "function" && l.removeEventListener(t, n, e), typeof u == "function")) {
              typeof n != "function" && n !== null && (a in l ? l[a] = null : l.hasAttribute(a) && l.removeAttribute(a)), l.addEventListener(t, u, e);
              break l;
            }
            a in l ? l[a] = u : u === !0 ? l.setAttribute(a, "") : je(l, a, u);
          }
    }
  }
  function Jl(l, t, a) {
    switch (t) {
      case "div":
      case "span":
      case "svg":
      case "path":
      case "a":
      case "g":
      case "p":
      case "li":
        break;
      case "img":
        w("error", l), w("load", l);
        var u = !1, e = !1, n;
        for (n in a)
          if (a.hasOwnProperty(n)) {
            var c = a[n];
            if (c != null)
              switch (n) {
                case "src":
                  u = !0;
                  break;
                case "srcSet":
                  e = !0;
                  break;
                case "children":
                case "dangerouslySetInnerHTML":
                  throw Error(d(137, t));
                default:
                  sl(l, t, n, c, a, null);
              }
          }
        e && sl(l, t, "srcSet", a.srcSet, a, null), u && sl(l, t, "src", a.src, a, null);
        return;
      case "input":
        w("invalid", l);
        var i = n = c = e = null, f = null, m = null;
        for (u in a)
          if (a.hasOwnProperty(u)) {
            var r = a[u];
            if (r != null)
              switch (u) {
                case "name":
                  e = r;
                  break;
                case "type":
                  c = r;
                  break;
                case "checked":
                  f = r;
                  break;
                case "defaultChecked":
                  m = r;
                  break;
                case "value":
                  n = r;
                  break;
                case "defaultValue":
                  i = r;
                  break;
                case "children":
                case "dangerouslySetInnerHTML":
                  if (r != null)
                    throw Error(d(137, t));
                  break;
                default:
                  sl(l, t, u, r, a, null);
              }
          }
        qf(
          l,
          n,
          i,
          f,
          m,
          c,
          e,
          !1
        );
        return;
      case "select":
        w("invalid", l), u = c = n = null;
        for (e in a)
          if (a.hasOwnProperty(e) && (i = a[e], i != null))
            switch (e) {
              case "value":
                n = i;
                break;
              case "defaultValue":
                c = i;
                break;
              case "multiple":
                u = i;
              default:
                sl(l, t, e, i, a, null);
            }
        t = n, a = c, l.multiple = !!u, t != null ? $a(l, !!u, t, !1) : a != null && $a(l, !!u, a, !0);
        return;
      case "textarea":
        w("invalid", l), n = e = u = null;
        for (c in a)
          if (a.hasOwnProperty(c) && (i = a[c], i != null))
            switch (c) {
              case "value":
                u = i;
                break;
              case "defaultValue":
                e = i;
                break;
              case "children":
                n = i;
                break;
              case "dangerouslySetInnerHTML":
                if (i != null) throw Error(d(91));
                break;
              default:
                sl(l, t, c, i, a, null);
            }
        Cf(l, u, e, n);
        return;
      case "option":
        for (f in a)
          if (a.hasOwnProperty(f) && (u = a[f], u != null))
            switch (f) {
              case "selected":
                l.selected = u && typeof u != "function" && typeof u != "symbol";
                break;
              default:
                sl(l, t, f, u, a, null);
            }
        return;
      case "dialog":
        w("beforetoggle", l), w("toggle", l), w("cancel", l), w("close", l);
        break;
      case "iframe":
      case "object":
        w("load", l);
        break;
      case "video":
      case "audio":
        for (u = 0; u < ye.length; u++)
          w(ye[u], l);
        break;
      case "image":
        w("error", l), w("load", l);
        break;
      case "details":
        w("toggle", l);
        break;
      case "embed":
      case "source":
      case "link":
        w("error", l), w("load", l);
      case "area":
      case "base":
      case "br":
      case "col":
      case "hr":
      case "keygen":
      case "meta":
      case "param":
      case "track":
      case "wbr":
      case "menuitem":
        for (m in a)
          if (a.hasOwnProperty(m) && (u = a[m], u != null))
            switch (m) {
              case "children":
              case "dangerouslySetInnerHTML":
                throw Error(d(137, t));
              default:
                sl(l, t, m, u, a, null);
            }
        return;
      default:
        if (ec(t)) {
          for (r in a)
            a.hasOwnProperty(r) && (u = a[r], u !== void 0 && Vi(
              l,
              t,
              r,
              u,
              a,
              void 0
            ));
          return;
        }
    }
    for (i in a)
      a.hasOwnProperty(i) && (u = a[i], u != null && sl(l, t, i, u, a, null));
  }
  function Ty(l, t, a, u) {
    switch (t) {
      case "div":
      case "span":
      case "svg":
      case "path":
      case "a":
      case "g":
      case "p":
      case "li":
        break;
      case "input":
        var e = null, n = null, c = null, i = null, f = null, m = null, r = null;
        for (g in a) {
          var z = a[g];
          if (a.hasOwnProperty(g) && z != null)
            switch (g) {
              case "checked":
                break;
              case "value":
                break;
              case "defaultValue":
                f = z;
              default:
                u.hasOwnProperty(g) || sl(l, t, g, null, u, z);
            }
        }
        for (var h in u) {
          var g = u[h];
          if (z = a[h], u.hasOwnProperty(h) && (g != null || z != null))
            switch (h) {
              case "type":
                n = g;
                break;
              case "name":
                e = g;
                break;
              case "checked":
                m = g;
                break;
              case "defaultChecked":
                r = g;
                break;
              case "value":
                c = g;
                break;
              case "defaultValue":
                i = g;
                break;
              case "children":
              case "dangerouslySetInnerHTML":
                if (g != null)
                  throw Error(d(137, t));
                break;
              default:
                g !== z && sl(
                  l,
                  t,
                  h,
                  g,
                  u,
                  z
                );
            }
        }
        ac(
          l,
          c,
          i,
          f,
          m,
          r,
          n,
          e
        );
        return;
      case "select":
        g = c = i = h = null;
        for (n in a)
          if (f = a[n], a.hasOwnProperty(n) && f != null)
            switch (n) {
              case "value":
                break;
              case "multiple":
                g = f;
              default:
                u.hasOwnProperty(n) || sl(
                  l,
                  t,
                  n,
                  null,
                  u,
                  f
                );
            }
        for (e in u)
          if (n = u[e], f = a[e], u.hasOwnProperty(e) && (n != null || f != null))
            switch (e) {
              case "value":
                h = n;
                break;
              case "defaultValue":
                i = n;
                break;
              case "multiple":
                c = n;
              default:
                n !== f && sl(
                  l,
                  t,
                  e,
                  n,
                  u,
                  f
                );
            }
        t = i, a = c, u = g, h != null ? $a(l, !!a, h, !1) : !!u != !!a && (t != null ? $a(l, !!a, t, !0) : $a(l, !!a, a ? [] : "", !1));
        return;
      case "textarea":
        g = h = null;
        for (i in a)
          if (e = a[i], a.hasOwnProperty(i) && e != null && !u.hasOwnProperty(i))
            switch (i) {
              case "value":
                break;
              case "children":
                break;
              default:
                sl(l, t, i, null, u, e);
            }
        for (c in u)
          if (e = u[c], n = a[c], u.hasOwnProperty(c) && (e != null || n != null))
            switch (c) {
              case "value":
                h = e;
                break;
              case "defaultValue":
                g = e;
                break;
              case "children":
                break;
              case "dangerouslySetInnerHTML":
                if (e != null) throw Error(d(91));
                break;
              default:
                e !== n && sl(l, t, c, e, u, n);
            }
        Bf(l, h, g);
        return;
      case "option":
        for (var N in a)
          if (h = a[N], a.hasOwnProperty(N) && h != null && !u.hasOwnProperty(N))
            switch (N) {
              case "selected":
                l.selected = !1;
                break;
              default:
                sl(
                  l,
                  t,
                  N,
                  null,
                  u,
                  h
                );
            }
        for (f in u)
          if (h = u[f], g = a[f], u.hasOwnProperty(f) && h !== g && (h != null || g != null))
            switch (f) {
              case "selected":
                l.selected = h && typeof h != "function" && typeof h != "symbol";
                break;
              default:
                sl(
                  l,
                  t,
                  f,
                  h,
                  u,
                  g
                );
            }
        return;
      case "img":
      case "link":
      case "area":
      case "base":
      case "br":
      case "col":
      case "embed":
      case "hr":
      case "keygen":
      case "meta":
      case "param":
      case "source":
      case "track":
      case "wbr":
      case "menuitem":
        for (var B in a)
          h = a[B], a.hasOwnProperty(B) && h != null && !u.hasOwnProperty(B) && sl(l, t, B, null, u, h);
        for (m in u)
          if (h = u[m], g = a[m], u.hasOwnProperty(m) && h !== g && (h != null || g != null))
            switch (m) {
              case "children":
              case "dangerouslySetInnerHTML":
                if (h != null)
                  throw Error(d(137, t));
                break;
              default:
                sl(
                  l,
                  t,
                  m,
                  h,
                  u,
                  g
                );
            }
        return;
      default:
        if (ec(t)) {
          for (var vl in a)
            h = a[vl], a.hasOwnProperty(vl) && h !== void 0 && !u.hasOwnProperty(vl) && Vi(
              l,
              t,
              vl,
              void 0,
              u,
              h
            );
          for (r in u)
            h = u[r], g = a[r], !u.hasOwnProperty(r) || h === g || h === void 0 && g === void 0 || Vi(
              l,
              t,
              r,
              h,
              u,
              g
            );
          return;
        }
    }
    for (var o in a)
      h = a[o], a.hasOwnProperty(o) && h != null && !u.hasOwnProperty(o) && sl(l, t, o, null, u, h);
    for (z in u)
      h = u[z], g = a[z], !u.hasOwnProperty(z) || h === g || h == null && g == null || sl(l, t, z, h, u, g);
  }
  function Qo(l) {
    switch (l) {
      case "css":
      case "script":
      case "font":
      case "img":
      case "image":
      case "input":
      case "link":
        return !0;
      default:
        return !1;
    }
  }
  function Ay() {
    if (typeof performance.getEntriesByType == "function") {
      for (var l = 0, t = 0, a = performance.getEntriesByType("resource"), u = 0; u < a.length; u++) {
        var e = a[u], n = e.transferSize, c = e.initiatorType, i = e.duration;
        if (n && i && Qo(c)) {
          for (c = 0, i = e.responseEnd, u += 1; u < a.length; u++) {
            var f = a[u], m = f.startTime;
            if (m > i) break;
            var r = f.transferSize, z = f.initiatorType;
            r && Qo(z) && (f = f.responseEnd, c += r * (f < i ? 1 : (i - m) / (f - m)));
          }
          if (--u, t += 8 * (n + c) / (e.duration / 1e3), l++, 10 < l) break;
        }
      }
      if (0 < l) return t / l / 1e6;
    }
    return navigator.connection && (l = navigator.connection.downlink, typeof l == "number") ? l : 5;
  }
  var Ki = null, Ji = null;
  function Nn(l) {
    return l.nodeType === 9 ? l : l.ownerDocument;
  }
  function Zo(l) {
    switch (l) {
      case "http://www.w3.org/2000/svg":
        return 1;
      case "http://www.w3.org/1998/Math/MathML":
        return 2;
      default:
        return 0;
    }
  }
  function Lo(l, t) {
    if (l === 0)
      switch (t) {
        case "svg":
          return 1;
        case "math":
          return 2;
        default:
          return 0;
      }
    return l === 1 && t === "foreignObject" ? 0 : l;
  }
  function wi(l, t) {
    return l === "textarea" || l === "noscript" || typeof t.children == "string" || typeof t.children == "number" || typeof t.children == "bigint" || typeof t.dangerouslySetInnerHTML == "object" && t.dangerouslySetInnerHTML !== null && t.dangerouslySetInnerHTML.__html != null;
  }
  var ki = null;
  function py() {
    var l = window.event;
    return l && l.type === "popstate" ? l === ki ? !1 : (ki = l, !0) : (ki = null, !1);
  }
  var Vo = typeof setTimeout == "function" ? setTimeout : void 0, My = typeof clearTimeout == "function" ? clearTimeout : void 0, Ko = typeof Promise == "function" ? Promise : void 0, Oy = typeof queueMicrotask == "function" ? queueMicrotask : typeof Ko < "u" ? function(l) {
    return Ko.resolve(null).then(l).catch(Dy);
  } : Vo;
  function Dy(l) {
    setTimeout(function() {
      throw l;
    });
  }
  function _a(l) {
    return l === "head";
  }
  function Jo(l, t) {
    var a = t, u = 0;
    do {
      var e = a.nextSibling;
      if (l.removeChild(a), e && e.nodeType === 8)
        if (a = e.data, a === "/$" || a === "/&") {
          if (u === 0) {
            l.removeChild(e), Du(t);
            return;
          }
          u--;
        } else if (a === "$" || a === "$?" || a === "$~" || a === "$!" || a === "&")
          u++;
        else if (a === "html")
          he(l.ownerDocument.documentElement);
        else if (a === "head") {
          a = l.ownerDocument.head, he(a);
          for (var n = a.firstChild; n; ) {
            var c = n.nextSibling, i = n.nodeName;
            n[Ru] || i === "SCRIPT" || i === "STYLE" || i === "LINK" && n.rel.toLowerCase() === "stylesheet" || a.removeChild(n), n = c;
          }
        } else
          a === "body" && he(l.ownerDocument.body);
      a = e;
    } while (a);
    Du(t);
  }
  function wo(l, t) {
    var a = l;
    l = 0;
    do {
      var u = a.nextSibling;
      if (a.nodeType === 1 ? t ? (a._stashedDisplay = a.style.display, a.style.display = "none") : (a.style.display = a._stashedDisplay || "", a.getAttribute("style") === "" && a.removeAttribute("style")) : a.nodeType === 3 && (t ? (a._stashedText = a.nodeValue, a.nodeValue = "") : a.nodeValue = a._stashedText || ""), u && u.nodeType === 8)
        if (a = u.data, a === "/$") {
          if (l === 0) break;
          l--;
        } else
          a !== "$" && a !== "$?" && a !== "$~" && a !== "$!" || l++;
      a = u;
    } while (a);
  }
  function Wi(l) {
    var t = l.firstChild;
    for (t && t.nodeType === 10 && (t = t.nextSibling); t; ) {
      var a = t;
      switch (t = t.nextSibling, a.nodeName) {
        case "HTML":
        case "HEAD":
        case "BODY":
          Wi(a), lc(a);
          continue;
        case "SCRIPT":
        case "STYLE":
          continue;
        case "LINK":
          if (a.rel.toLowerCase() === "stylesheet") continue;
      }
      l.removeChild(a);
    }
  }
  function Uy(l, t, a, u) {
    for (; l.nodeType === 1; ) {
      var e = a;
      if (l.nodeName.toLowerCase() !== t.toLowerCase()) {
        if (!u && (l.nodeName !== "INPUT" || l.type !== "hidden"))
          break;
      } else if (u) {
        if (!l[Ru])
          switch (t) {
            case "meta":
              if (!l.hasAttribute("itemprop")) break;
              return l;
            case "link":
              if (n = l.getAttribute("rel"), n === "stylesheet" && l.hasAttribute("data-precedence"))
                break;
              if (n !== e.rel || l.getAttribute("href") !== (e.href == null || e.href === "" ? null : e.href) || l.getAttribute("crossorigin") !== (e.crossOrigin == null ? null : e.crossOrigin) || l.getAttribute("title") !== (e.title == null ? null : e.title))
                break;
              return l;
            case "style":
              if (l.hasAttribute("data-precedence")) break;
              return l;
            case "script":
              if (n = l.getAttribute("src"), (n !== (e.src == null ? null : e.src) || l.getAttribute("type") !== (e.type == null ? null : e.type) || l.getAttribute("crossorigin") !== (e.crossOrigin == null ? null : e.crossOrigin)) && n && l.hasAttribute("async") && !l.hasAttribute("itemprop"))
                break;
              return l;
            default:
              return l;
          }
      } else if (t === "input" && l.type === "hidden") {
        var n = e.name == null ? null : "" + e.name;
        if (e.type === "hidden" && l.getAttribute("name") === n)
          return l;
      } else return l;
      if (l = pt(l.nextSibling), l === null) break;
    }
    return null;
  }
  function Ny(l, t, a) {
    if (t === "") return null;
    for (; l.nodeType !== 3; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !a || (l = pt(l.nextSibling), l === null)) return null;
    return l;
  }
  function ko(l, t) {
    for (; l.nodeType !== 8; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !t || (l = pt(l.nextSibling), l === null)) return null;
    return l;
  }
  function $i(l) {
    return l.data === "$?" || l.data === "$~";
  }
  function Fi(l) {
    return l.data === "$!" || l.data === "$?" && l.ownerDocument.readyState !== "loading";
  }
  function jy(l, t) {
    var a = l.ownerDocument;
    if (l.data === "$~") l._reactRetry = t;
    else if (l.data !== "$?" || a.readyState !== "loading")
      t();
    else {
      var u = function() {
        t(), a.removeEventListener("DOMContentLoaded", u);
      };
      a.addEventListener("DOMContentLoaded", u), l._reactRetry = u;
    }
  }
  function pt(l) {
    for (; l != null; l = l.nextSibling) {
      var t = l.nodeType;
      if (t === 1 || t === 3) break;
      if (t === 8) {
        if (t = l.data, t === "$" || t === "$!" || t === "$?" || t === "$~" || t === "&" || t === "F!" || t === "F")
          break;
        if (t === "/$" || t === "/&") return null;
      }
    }
    return l;
  }
  var Ii = null;
  function Wo(l) {
    l = l.nextSibling;
    for (var t = 0; l; ) {
      if (l.nodeType === 8) {
        var a = l.data;
        if (a === "/$" || a === "/&") {
          if (t === 0)
            return pt(l.nextSibling);
          t--;
        } else
          a !== "$" && a !== "$!" && a !== "$?" && a !== "$~" && a !== "&" || t++;
      }
      l = l.nextSibling;
    }
    return null;
  }
  function $o(l) {
    l = l.previousSibling;
    for (var t = 0; l; ) {
      if (l.nodeType === 8) {
        var a = l.data;
        if (a === "$" || a === "$!" || a === "$?" || a === "$~" || a === "&") {
          if (t === 0) return l;
          t--;
        } else a !== "/$" && a !== "/&" || t++;
      }
      l = l.previousSibling;
    }
    return null;
  }
  function Fo(l, t, a) {
    switch (t = Nn(a), l) {
      case "html":
        if (l = t.documentElement, !l) throw Error(d(452));
        return l;
      case "head":
        if (l = t.head, !l) throw Error(d(453));
        return l;
      case "body":
        if (l = t.body, !l) throw Error(d(454));
        return l;
      default:
        throw Error(d(451));
    }
  }
  function he(l) {
    for (var t = l.attributes; t.length; )
      l.removeAttributeNode(t[0]);
    lc(l);
  }
  var Mt = /* @__PURE__ */ new Map(), Io = /* @__PURE__ */ new Set();
  function jn(l) {
    return typeof l.getRootNode == "function" ? l.getRootNode() : l.nodeType === 9 ? l : l.ownerDocument;
  }
  var la = M.d;
  M.d = {
    f: Hy,
    r: Ry,
    D: xy,
    C: qy,
    L: By,
    m: Cy,
    X: Gy,
    S: Yy,
    M: Xy
  };
  function Hy() {
    var l = la.f(), t = En();
    return l || t;
  }
  function Ry(l) {
    var t = wa(l);
    t !== null && t.tag === 5 && t.type === "form" ? mv(t) : la.r(l);
  }
  var pu = typeof document > "u" ? null : document;
  function Po(l, t, a) {
    var u = pu;
    if (u && typeof t == "string" && t) {
      var e = rt(t);
      e = 'link[rel="' + l + '"][href="' + e + '"]', typeof a == "string" && (e += '[crossorigin="' + a + '"]'), Io.has(e) || (Io.add(e), l = { rel: l, crossOrigin: a, href: t }, u.querySelector(e) === null && (t = u.createElement("link"), Jl(t, "link", l), ql(t), u.head.appendChild(t)));
    }
  }
  function xy(l) {
    la.D(l), Po("dns-prefetch", l, null);
  }
  function qy(l, t) {
    la.C(l, t), Po("preconnect", l, t);
  }
  function By(l, t, a) {
    la.L(l, t, a);
    var u = pu;
    if (u && l && t) {
      var e = 'link[rel="preload"][as="' + rt(t) + '"]';
      t === "image" && a && a.imageSrcSet ? (e += '[imagesrcset="' + rt(
        a.imageSrcSet
      ) + '"]', typeof a.imageSizes == "string" && (e += '[imagesizes="' + rt(
        a.imageSizes
      ) + '"]')) : e += '[href="' + rt(l) + '"]';
      var n = e;
      switch (t) {
        case "style":
          n = Mu(l);
          break;
        case "script":
          n = Ou(l);
      }
      Mt.has(n) || (l = x(
        {
          rel: "preload",
          href: t === "image" && a && a.imageSrcSet ? void 0 : l,
          as: t
        },
        a
      ), Mt.set(n, l), u.querySelector(e) !== null || t === "style" && u.querySelector(ge(n)) || t === "script" && u.querySelector(Se(n)) || (t = u.createElement("link"), Jl(t, "link", l), ql(t), u.head.appendChild(t)));
    }
  }
  function Cy(l, t) {
    la.m(l, t);
    var a = pu;
    if (a && l) {
      var u = t && typeof t.as == "string" ? t.as : "script", e = 'link[rel="modulepreload"][as="' + rt(u) + '"][href="' + rt(l) + '"]', n = e;
      switch (u) {
        case "audioworklet":
        case "paintworklet":
        case "serviceworker":
        case "sharedworker":
        case "worker":
        case "script":
          n = Ou(l);
      }
      if (!Mt.has(n) && (l = x({ rel: "modulepreload", href: l }, t), Mt.set(n, l), a.querySelector(e) === null)) {
        switch (u) {
          case "audioworklet":
          case "paintworklet":
          case "serviceworker":
          case "sharedworker":
          case "worker":
          case "script":
            if (a.querySelector(Se(n)))
              return;
        }
        u = a.createElement("link"), Jl(u, "link", l), ql(u), a.head.appendChild(u);
      }
    }
  }
  function Yy(l, t, a) {
    la.S(l, t, a);
    var u = pu;
    if (u && l) {
      var e = ka(u).hoistableStyles, n = Mu(l);
      t = t || "default";
      var c = e.get(n);
      if (!c) {
        var i = { loading: 0, preload: null };
        if (c = u.querySelector(
          ge(n)
        ))
          i.loading = 5;
        else {
          l = x(
            { rel: "stylesheet", href: l, "data-precedence": t },
            a
          ), (a = Mt.get(n)) && Pi(l, a);
          var f = c = u.createElement("link");
          ql(f), Jl(f, "link", l), f._p = new Promise(function(m, r) {
            f.onload = m, f.onerror = r;
          }), f.addEventListener("load", function() {
            i.loading |= 1;
          }), f.addEventListener("error", function() {
            i.loading |= 2;
          }), i.loading |= 4, Hn(c, t, u);
        }
        c = {
          type: "stylesheet",
          instance: c,
          count: 1,
          state: i
        }, e.set(n, c);
      }
    }
  }
  function Gy(l, t) {
    la.X(l, t);
    var a = pu;
    if (a && l) {
      var u = ka(a).hoistableScripts, e = Ou(l), n = u.get(e);
      n || (n = a.querySelector(Se(e)), n || (l = x({ src: l, async: !0 }, t), (t = Mt.get(e)) && lf(l, t), n = a.createElement("script"), ql(n), Jl(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, u.set(e, n));
    }
  }
  function Xy(l, t) {
    la.M(l, t);
    var a = pu;
    if (a && l) {
      var u = ka(a).hoistableScripts, e = Ou(l), n = u.get(e);
      n || (n = a.querySelector(Se(e)), n || (l = x({ src: l, async: !0, type: "module" }, t), (t = Mt.get(e)) && lf(l, t), n = a.createElement("script"), ql(n), Jl(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, u.set(e, n));
    }
  }
  function ld(l, t, a, u) {
    var e = (e = K.current) ? jn(e) : null;
    if (!e) throw Error(d(446));
    switch (l) {
      case "meta":
      case "title":
        return null;
      case "style":
        return typeof a.precedence == "string" && typeof a.href == "string" ? (t = Mu(a.href), a = ka(
          e
        ).hoistableStyles, u = a.get(t), u || (u = {
          type: "style",
          instance: null,
          count: 0,
          state: null
        }, a.set(t, u)), u) : { type: "void", instance: null, count: 0, state: null };
      case "link":
        if (a.rel === "stylesheet" && typeof a.href == "string" && typeof a.precedence == "string") {
          l = Mu(a.href);
          var n = ka(
            e
          ).hoistableStyles, c = n.get(l);
          if (c || (e = e.ownerDocument || e, c = {
            type: "stylesheet",
            instance: null,
            count: 0,
            state: { loading: 0, preload: null }
          }, n.set(l, c), (n = e.querySelector(
            ge(l)
          )) && !n._p && (c.instance = n, c.state.loading = 5), Mt.has(l) || (a = {
            rel: "preload",
            as: "style",
            href: a.href,
            crossOrigin: a.crossOrigin,
            integrity: a.integrity,
            media: a.media,
            hrefLang: a.hrefLang,
            referrerPolicy: a.referrerPolicy
          }, Mt.set(l, a), n || Qy(
            e,
            l,
            a,
            c.state
          ))), t && u === null)
            throw Error(d(528, ""));
          return c;
        }
        if (t && u !== null)
          throw Error(d(529, ""));
        return null;
      case "script":
        return t = a.async, a = a.src, typeof a == "string" && t && typeof t != "function" && typeof t != "symbol" ? (t = Ou(a), a = ka(
          e
        ).hoistableScripts, u = a.get(t), u || (u = {
          type: "script",
          instance: null,
          count: 0,
          state: null
        }, a.set(t, u)), u) : { type: "void", instance: null, count: 0, state: null };
      default:
        throw Error(d(444, l));
    }
  }
  function Mu(l) {
    return 'href="' + rt(l) + '"';
  }
  function ge(l) {
    return 'link[rel="stylesheet"][' + l + "]";
  }
  function td(l) {
    return x({}, l, {
      "data-precedence": l.precedence,
      precedence: null
    });
  }
  function Qy(l, t, a, u) {
    l.querySelector('link[rel="preload"][as="style"][' + t + "]") ? u.loading = 1 : (t = l.createElement("link"), u.preload = t, t.addEventListener("load", function() {
      return u.loading |= 1;
    }), t.addEventListener("error", function() {
      return u.loading |= 2;
    }), Jl(t, "link", a), ql(t), l.head.appendChild(t));
  }
  function Ou(l) {
    return '[src="' + rt(l) + '"]';
  }
  function Se(l) {
    return "script[async]" + l;
  }
  function ad(l, t, a) {
    if (t.count++, t.instance === null)
      switch (t.type) {
        case "style":
          var u = l.querySelector(
            'style[data-href~="' + rt(a.href) + '"]'
          );
          if (u)
            return t.instance = u, ql(u), u;
          var e = x({}, a, {
            "data-href": a.href,
            "data-precedence": a.precedence,
            href: null,
            precedence: null
          });
          return u = (l.ownerDocument || l).createElement(
            "style"
          ), ql(u), Jl(u, "style", e), Hn(u, a.precedence, l), t.instance = u;
        case "stylesheet":
          e = Mu(a.href);
          var n = l.querySelector(
            ge(e)
          );
          if (n)
            return t.state.loading |= 4, t.instance = n, ql(n), n;
          u = td(a), (e = Mt.get(e)) && Pi(u, e), n = (l.ownerDocument || l).createElement("link"), ql(n);
          var c = n;
          return c._p = new Promise(function(i, f) {
            c.onload = i, c.onerror = f;
          }), Jl(n, "link", u), t.state.loading |= 4, Hn(n, a.precedence, l), t.instance = n;
        case "script":
          return n = Ou(a.src), (e = l.querySelector(
            Se(n)
          )) ? (t.instance = e, ql(e), e) : (u = a, (e = Mt.get(n)) && (u = x({}, a), lf(u, e)), l = l.ownerDocument || l, e = l.createElement("script"), ql(e), Jl(e, "link", u), l.head.appendChild(e), t.instance = e);
        case "void":
          return null;
        default:
          throw Error(d(443, t.type));
      }
    else
      t.type === "stylesheet" && (t.state.loading & 4) === 0 && (u = t.instance, t.state.loading |= 4, Hn(u, a.precedence, l));
    return t.instance;
  }
  function Hn(l, t, a) {
    for (var u = a.querySelectorAll(
      'link[rel="stylesheet"][data-precedence],style[data-precedence]'
    ), e = u.length ? u[u.length - 1] : null, n = e, c = 0; c < u.length; c++) {
      var i = u[c];
      if (i.dataset.precedence === t) n = i;
      else if (n !== e) break;
    }
    n ? n.parentNode.insertBefore(l, n.nextSibling) : (t = a.nodeType === 9 ? a.head : a, t.insertBefore(l, t.firstChild));
  }
  function Pi(l, t) {
    l.crossOrigin == null && (l.crossOrigin = t.crossOrigin), l.referrerPolicy == null && (l.referrerPolicy = t.referrerPolicy), l.title == null && (l.title = t.title);
  }
  function lf(l, t) {
    l.crossOrigin == null && (l.crossOrigin = t.crossOrigin), l.referrerPolicy == null && (l.referrerPolicy = t.referrerPolicy), l.integrity == null && (l.integrity = t.integrity);
  }
  var Rn = null;
  function ud(l, t, a) {
    if (Rn === null) {
      var u = /* @__PURE__ */ new Map(), e = Rn = /* @__PURE__ */ new Map();
      e.set(a, u);
    } else
      e = Rn, u = e.get(a), u || (u = /* @__PURE__ */ new Map(), e.set(a, u));
    if (u.has(l)) return u;
    for (u.set(l, null), a = a.getElementsByTagName(l), e = 0; e < a.length; e++) {
      var n = a[e];
      if (!(n[Ru] || n[Zl] || l === "link" && n.getAttribute("rel") === "stylesheet") && n.namespaceURI !== "http://www.w3.org/2000/svg") {
        var c = n.getAttribute(t) || "";
        c = l + c;
        var i = u.get(c);
        i ? i.push(n) : u.set(c, [n]);
      }
    }
    return u;
  }
  function ed(l, t, a) {
    l = l.ownerDocument || l, l.head.insertBefore(
      a,
      t === "title" ? l.querySelector("head > title") : null
    );
  }
  function Zy(l, t, a) {
    if (a === 1 || t.itemProp != null) return !1;
    switch (l) {
      case "meta":
      case "title":
        return !0;
      case "style":
        if (typeof t.precedence != "string" || typeof t.href != "string" || t.href === "")
          break;
        return !0;
      case "link":
        if (typeof t.rel != "string" || typeof t.href != "string" || t.href === "" || t.onLoad || t.onError)
          break;
        switch (t.rel) {
          case "stylesheet":
            return l = t.disabled, typeof t.precedence == "string" && l == null;
          default:
            return !0;
        }
      case "script":
        if (t.async && typeof t.async != "function" && typeof t.async != "symbol" && !t.onLoad && !t.onError && t.src && typeof t.src == "string")
          return !0;
    }
    return !1;
  }
  function nd(l) {
    return !(l.type === "stylesheet" && (l.state.loading & 3) === 0);
  }
  function Ly(l, t, a, u) {
    if (a.type === "stylesheet" && (typeof u.media != "string" || matchMedia(u.media).matches !== !1) && (a.state.loading & 4) === 0) {
      if (a.instance === null) {
        var e = Mu(u.href), n = t.querySelector(
          ge(e)
        );
        if (n) {
          t = n._p, t !== null && typeof t == "object" && typeof t.then == "function" && (l.count++, l = xn.bind(l), t.then(l, l)), a.state.loading |= 4, a.instance = n, ql(n);
          return;
        }
        n = t.ownerDocument || t, u = td(u), (e = Mt.get(e)) && Pi(u, e), n = n.createElement("link"), ql(n);
        var c = n;
        c._p = new Promise(function(i, f) {
          c.onload = i, c.onerror = f;
        }), Jl(n, "link", u), a.instance = n;
      }
      l.stylesheets === null && (l.stylesheets = /* @__PURE__ */ new Map()), l.stylesheets.set(a, t), (t = a.state.preload) && (a.state.loading & 3) === 0 && (l.count++, a = xn.bind(l), t.addEventListener("load", a), t.addEventListener("error", a));
    }
  }
  var tf = 0;
  function Vy(l, t) {
    return l.stylesheets && l.count === 0 && Bn(l, l.stylesheets), 0 < l.count || 0 < l.imgCount ? function(a) {
      var u = setTimeout(function() {
        if (l.stylesheets && Bn(l, l.stylesheets), l.unsuspend) {
          var n = l.unsuspend;
          l.unsuspend = null, n();
        }
      }, 6e4 + t);
      0 < l.imgBytes && tf === 0 && (tf = 62500 * Ay());
      var e = setTimeout(
        function() {
          if (l.waitingForImages = !1, l.count === 0 && (l.stylesheets && Bn(l, l.stylesheets), l.unsuspend)) {
            var n = l.unsuspend;
            l.unsuspend = null, n();
          }
        },
        (l.imgBytes > tf ? 50 : 800) + t
      );
      return l.unsuspend = a, function() {
        l.unsuspend = null, clearTimeout(u), clearTimeout(e);
      };
    } : null;
  }
  function xn() {
    if (this.count--, this.count === 0 && (this.imgCount === 0 || !this.waitingForImages)) {
      if (this.stylesheets) Bn(this, this.stylesheets);
      else if (this.unsuspend) {
        var l = this.unsuspend;
        this.unsuspend = null, l();
      }
    }
  }
  var qn = null;
  function Bn(l, t) {
    l.stylesheets = null, l.unsuspend !== null && (l.count++, qn = /* @__PURE__ */ new Map(), t.forEach(Ky, l), qn = null, xn.call(l));
  }
  function Ky(l, t) {
    if (!(t.state.loading & 4)) {
      var a = qn.get(l);
      if (a) var u = a.get(null);
      else {
        a = /* @__PURE__ */ new Map(), qn.set(l, a);
        for (var e = l.querySelectorAll(
          "link[data-precedence],style[data-precedence]"
        ), n = 0; n < e.length; n++) {
          var c = e[n];
          (c.nodeName === "LINK" || c.getAttribute("media") !== "not all") && (a.set(c.dataset.precedence, c), u = c);
        }
        u && a.set(null, u);
      }
      e = t.instance, c = e.getAttribute("data-precedence"), n = a.get(c) || u, n === u && a.set(null, e), a.set(c, e), this.count++, u = xn.bind(this), e.addEventListener("load", u), e.addEventListener("error", u), n ? n.parentNode.insertBefore(e, n.nextSibling) : (l = l.nodeType === 9 ? l.head : l, l.insertBefore(e, l.firstChild)), t.state.loading |= 4;
    }
  }
  var re = {
    $$typeof: Tl,
    Provider: null,
    Consumer: null,
    _currentValue: C,
    _currentValue2: C,
    _threadCount: 0
  };
  function Jy(l, t, a, u, e, n, c, i, f) {
    this.tag = 1, this.containerInfo = l, this.pingCache = this.current = this.pendingChildren = null, this.timeoutHandle = -1, this.callbackNode = this.next = this.pendingContext = this.context = this.cancelPendingCommit = null, this.callbackPriority = 0, this.expirationTimes = $n(-1), this.entangledLanes = this.shellSuspendCounter = this.errorRecoveryDisabledLanes = this.expiredLanes = this.warmLanes = this.pingedLanes = this.suspendedLanes = this.pendingLanes = 0, this.entanglements = $n(0), this.hiddenUpdates = $n(null), this.identifierPrefix = u, this.onUncaughtError = e, this.onCaughtError = n, this.onRecoverableError = c, this.pooledCache = null, this.pooledCacheLanes = 0, this.formState = f, this.incompleteTransitions = /* @__PURE__ */ new Map();
  }
  function cd(l, t, a, u, e, n, c, i, f, m, r, z) {
    return l = new Jy(
      l,
      t,
      a,
      c,
      f,
      m,
      r,
      z,
      i
    ), t = 1, n === !0 && (t |= 24), n = ot(3, null, null, t), l.current = n, n.stateNode = l, t = xc(), t.refCount++, l.pooledCache = t, t.refCount++, n.memoizedState = {
      element: u,
      isDehydrated: a,
      cache: t
    }, Yc(n), l;
  }
  function id(l) {
    return l ? (l = eu, l) : eu;
  }
  function fd(l, t, a, u, e, n) {
    e = id(e), u.context === null ? u.context = e : u.pendingContext = e, u = sa(t), u.payload = { element: a }, n = n === void 0 ? null : n, n !== null && (u.callback = n), a = va(l, u, t), a !== null && (ut(a, l, t), $u(a, l, t));
  }
  function sd(l, t) {
    if (l = l.memoizedState, l !== null && l.dehydrated !== null) {
      var a = l.retryLane;
      l.retryLane = a !== 0 && a < t ? a : t;
    }
  }
  function af(l, t) {
    sd(l, t), (l = l.alternate) && sd(l, t);
  }
  function vd(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = Ha(l, 67108864);
      t !== null && ut(t, l, 67108864), af(l, 67108864);
    }
  }
  function od(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = gt();
      t = Fn(t);
      var a = Ha(l, t);
      a !== null && ut(a, l, t), af(l, t);
    }
  }
  var Cn = !0;
  function wy(l, t, a, u) {
    var e = S.T;
    S.T = null;
    var n = M.p;
    try {
      M.p = 2, uf(l, t, a, u);
    } finally {
      M.p = n, S.T = e;
    }
  }
  function ky(l, t, a, u) {
    var e = S.T;
    S.T = null;
    var n = M.p;
    try {
      M.p = 8, uf(l, t, a, u);
    } finally {
      M.p = n, S.T = e;
    }
  }
  function uf(l, t, a, u) {
    if (Cn) {
      var e = ef(u);
      if (e === null)
        Li(
          l,
          t,
          u,
          Yn,
          a
        ), yd(l, u);
      else if ($y(
        e,
        l,
        t,
        a,
        u
      ))
        u.stopPropagation();
      else if (yd(l, u), t & 4 && -1 < Wy.indexOf(l)) {
        for (; e !== null; ) {
          var n = wa(e);
          if (n !== null)
            switch (n.tag) {
              case 3:
                if (n = n.stateNode, n.current.memoizedState.isDehydrated) {
                  var c = Oa(n.pendingLanes);
                  if (c !== 0) {
                    var i = n;
                    for (i.pendingLanes |= 2, i.entangledLanes |= 2; c; ) {
                      var f = 1 << 31 - st(c);
                      i.entanglements[1] |= f, c &= ~f;
                    }
                    xt(n), (P & 6) === 0 && (_n = it() + 500, de(0));
                  }
                }
                break;
              case 31:
              case 13:
                i = Ha(n, 2), i !== null && ut(i, n, 2), En(), af(n, 2);
            }
          if (n = ef(u), n === null && Li(
            l,
            t,
            u,
            Yn,
            a
          ), n === e) break;
          e = n;
        }
        e !== null && u.stopPropagation();
      } else
        Li(
          l,
          t,
          u,
          null,
          a
        );
    }
  }
  function ef(l) {
    return l = cc(l), nf(l);
  }
  var Yn = null;
  function nf(l) {
    if (Yn = null, l = Ja(l), l !== null) {
      var t = yl(l);
      if (t === null) l = null;
      else {
        var a = t.tag;
        if (a === 13) {
          if (l = ul(t), l !== null) return l;
          l = null;
        } else if (a === 31) {
          if (l = ml(t), l !== null) return l;
          l = null;
        } else if (a === 3) {
          if (t.stateNode.current.memoizedState.isDehydrated)
            return t.tag === 3 ? t.stateNode.containerInfo : null;
          l = null;
        } else t !== l && (l = null);
      }
    }
    return Yn = l, null;
  }
  function dd(l) {
    switch (l) {
      case "beforetoggle":
      case "cancel":
      case "click":
      case "close":
      case "contextmenu":
      case "copy":
      case "cut":
      case "auxclick":
      case "dblclick":
      case "dragend":
      case "dragstart":
      case "drop":
      case "focusin":
      case "focusout":
      case "input":
      case "invalid":
      case "keydown":
      case "keypress":
      case "keyup":
      case "mousedown":
      case "mouseup":
      case "paste":
      case "pause":
      case "play":
      case "pointercancel":
      case "pointerdown":
      case "pointerup":
      case "ratechange":
      case "reset":
      case "resize":
      case "seeked":
      case "submit":
      case "toggle":
      case "touchcancel":
      case "touchend":
      case "touchstart":
      case "volumechange":
      case "change":
      case "selectionchange":
      case "textInput":
      case "compositionstart":
      case "compositionend":
      case "compositionupdate":
      case "beforeblur":
      case "afterblur":
      case "beforeinput":
      case "blur":
      case "fullscreenchange":
      case "focus":
      case "hashchange":
      case "popstate":
      case "select":
      case "selectstart":
        return 2;
      case "drag":
      case "dragenter":
      case "dragexit":
      case "dragleave":
      case "dragover":
      case "mousemove":
      case "mouseout":
      case "mouseover":
      case "pointermove":
      case "pointerout":
      case "pointerover":
      case "scroll":
      case "touchmove":
      case "wheel":
      case "mouseenter":
      case "mouseleave":
      case "pointerenter":
      case "pointerleave":
        return 8;
      case "message":
        switch (xd()) {
          case bf:
            return 2;
          case _f:
            return 8;
          case Me:
          case qd:
            return 32;
          case zf:
            return 268435456;
          default:
            return 32;
        }
      default:
        return 32;
    }
  }
  var cf = !1, za = null, Ea = null, Ta = null, be = /* @__PURE__ */ new Map(), _e = /* @__PURE__ */ new Map(), Aa = [], Wy = "mousedown mouseup touchcancel touchend touchstart auxclick dblclick pointercancel pointerdown pointerup dragend dragstart drop compositionend compositionstart keydown keypress keyup input textInput copy cut paste click change contextmenu reset".split(
    " "
  );
  function yd(l, t) {
    switch (l) {
      case "focusin":
      case "focusout":
        za = null;
        break;
      case "dragenter":
      case "dragleave":
        Ea = null;
        break;
      case "mouseover":
      case "mouseout":
        Ta = null;
        break;
      case "pointerover":
      case "pointerout":
        be.delete(t.pointerId);
        break;
      case "gotpointercapture":
      case "lostpointercapture":
        _e.delete(t.pointerId);
    }
  }
  function ze(l, t, a, u, e, n) {
    return l === null || l.nativeEvent !== n ? (l = {
      blockedOn: t,
      domEventName: a,
      eventSystemFlags: u,
      nativeEvent: n,
      targetContainers: [e]
    }, t !== null && (t = wa(t), t !== null && vd(t)), l) : (l.eventSystemFlags |= u, t = l.targetContainers, e !== null && t.indexOf(e) === -1 && t.push(e), l);
  }
  function $y(l, t, a, u, e) {
    switch (t) {
      case "focusin":
        return za = ze(
          za,
          l,
          t,
          a,
          u,
          e
        ), !0;
      case "dragenter":
        return Ea = ze(
          Ea,
          l,
          t,
          a,
          u,
          e
        ), !0;
      case "mouseover":
        return Ta = ze(
          Ta,
          l,
          t,
          a,
          u,
          e
        ), !0;
      case "pointerover":
        var n = e.pointerId;
        return be.set(
          n,
          ze(
            be.get(n) || null,
            l,
            t,
            a,
            u,
            e
          )
        ), !0;
      case "gotpointercapture":
        return n = e.pointerId, _e.set(
          n,
          ze(
            _e.get(n) || null,
            l,
            t,
            a,
            u,
            e
          )
        ), !0;
    }
    return !1;
  }
  function md(l) {
    var t = Ja(l.target);
    if (t !== null) {
      var a = yl(t);
      if (a !== null) {
        if (t = a.tag, t === 13) {
          if (t = ul(a), t !== null) {
            l.blockedOn = t, Of(l.priority, function() {
              od(a);
            });
            return;
          }
        } else if (t === 31) {
          if (t = ml(a), t !== null) {
            l.blockedOn = t, Of(l.priority, function() {
              od(a);
            });
            return;
          }
        } else if (t === 3 && a.stateNode.current.memoizedState.isDehydrated) {
          l.blockedOn = a.tag === 3 ? a.stateNode.containerInfo : null;
          return;
        }
      }
    }
    l.blockedOn = null;
  }
  function Gn(l) {
    if (l.blockedOn !== null) return !1;
    for (var t = l.targetContainers; 0 < t.length; ) {
      var a = ef(l.nativeEvent);
      if (a === null) {
        a = l.nativeEvent;
        var u = new a.constructor(
          a.type,
          a
        );
        nc = u, a.target.dispatchEvent(u), nc = null;
      } else
        return t = wa(a), t !== null && vd(t), l.blockedOn = a, !1;
      t.shift();
    }
    return !0;
  }
  function hd(l, t, a) {
    Gn(l) && a.delete(t);
  }
  function Fy() {
    cf = !1, za !== null && Gn(za) && (za = null), Ea !== null && Gn(Ea) && (Ea = null), Ta !== null && Gn(Ta) && (Ta = null), be.forEach(hd), _e.forEach(hd);
  }
  function Xn(l, t) {
    l.blockedOn === t && (l.blockedOn = null, cf || (cf = !0, _.unstable_scheduleCallback(
      _.unstable_NormalPriority,
      Fy
    )));
  }
  var Qn = null;
  function gd(l) {
    Qn !== l && (Qn = l, _.unstable_scheduleCallback(
      _.unstable_NormalPriority,
      function() {
        Qn === l && (Qn = null);
        for (var t = 0; t < l.length; t += 3) {
          var a = l[t], u = l[t + 1], e = l[t + 2];
          if (typeof u != "function") {
            if (nf(u || a) === null)
              continue;
            break;
          }
          var n = wa(a);
          n !== null && (l.splice(t, 3), t -= 3, ei(
            n,
            {
              pending: !0,
              data: e,
              method: a.method,
              action: u
            },
            u,
            e
          ));
        }
      }
    ));
  }
  function Du(l) {
    function t(f) {
      return Xn(f, l);
    }
    za !== null && Xn(za, l), Ea !== null && Xn(Ea, l), Ta !== null && Xn(Ta, l), be.forEach(t), _e.forEach(t);
    for (var a = 0; a < Aa.length; a++) {
      var u = Aa[a];
      u.blockedOn === l && (u.blockedOn = null);
    }
    for (; 0 < Aa.length && (a = Aa[0], a.blockedOn === null); )
      md(a), a.blockedOn === null && Aa.shift();
    if (a = (l.ownerDocument || l).$$reactFormReplay, a != null)
      for (u = 0; u < a.length; u += 3) {
        var e = a[u], n = a[u + 1], c = e[Fl] || null;
        if (typeof n == "function")
          c || gd(a);
        else if (c) {
          var i = null;
          if (n && n.hasAttribute("formAction")) {
            if (e = n, c = n[Fl] || null)
              i = c.formAction;
            else if (nf(e) !== null) continue;
          } else i = c.action;
          typeof i == "function" ? a[u + 1] = i : (a.splice(u, 3), u -= 3), gd(a);
        }
      }
  }
  function Sd() {
    function l(n) {
      n.canIntercept && n.info === "react-transition" && n.intercept({
        handler: function() {
          return new Promise(function(c) {
            return e = c;
          });
        },
        focusReset: "manual",
        scroll: "manual"
      });
    }
    function t() {
      e !== null && (e(), e = null), u || setTimeout(a, 20);
    }
    function a() {
      if (!u && !navigation.transition) {
        var n = navigation.currentEntry;
        n && n.url != null && navigation.navigate(n.url, {
          state: n.getState(),
          info: "react-transition",
          history: "replace"
        });
      }
    }
    if (typeof navigation == "object") {
      var u = !1, e = null;
      return navigation.addEventListener("navigate", l), navigation.addEventListener("navigatesuccess", t), navigation.addEventListener("navigateerror", t), setTimeout(a, 100), function() {
        u = !0, navigation.removeEventListener("navigate", l), navigation.removeEventListener("navigatesuccess", t), navigation.removeEventListener("navigateerror", t), e !== null && (e(), e = null);
      };
    }
  }
  function ff(l) {
    this._internalRoot = l;
  }
  Zn.prototype.render = ff.prototype.render = function(l) {
    var t = this._internalRoot;
    if (t === null) throw Error(d(409));
    var a = t.current, u = gt();
    fd(a, u, l, t, null, null);
  }, Zn.prototype.unmount = ff.prototype.unmount = function() {
    var l = this._internalRoot;
    if (l !== null) {
      this._internalRoot = null;
      var t = l.containerInfo;
      fd(l.current, 2, null, l, null, null), En(), t[Ka] = null;
    }
  };
  function Zn(l) {
    this._internalRoot = l;
  }
  Zn.prototype.unstable_scheduleHydration = function(l) {
    if (l) {
      var t = Mf();
      l = { blockedOn: null, target: l, priority: t };
      for (var a = 0; a < Aa.length && t !== 0 && t < Aa[a].priority; a++) ;
      Aa.splice(a, 0, l), a === 0 && md(l);
    }
  };
  var rd = D.version;
  if (rd !== "19.2.4")
    throw Error(
      d(
        527,
        rd,
        "19.2.4"
      )
    );
  M.findDOMNode = function(l) {
    var t = l._reactInternals;
    if (t === void 0)
      throw typeof l.render == "function" ? Error(d(188)) : (l = Object.keys(l).join(","), Error(d(268, l)));
    return l = T(t), l = l !== null ? Z(l) : null, l = l === null ? null : l.stateNode, l;
  };
  var Iy = {
    bundleType: 0,
    version: "19.2.4",
    rendererPackageName: "react-dom",
    currentDispatcherRef: S,
    reconcilerVersion: "19.2.4"
  };
  if (typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ < "u") {
    var Ln = __REACT_DEVTOOLS_GLOBAL_HOOK__;
    if (!Ln.isDisabled && Ln.supportsFiber)
      try {
        Nu = Ln.inject(
          Iy
        ), ft = Ln;
      } catch {
      }
  }
  return Te.createRoot = function(l, t) {
    if (!L(l)) throw Error(d(299));
    var a = !1, u = "", e = Av, n = pv, c = Mv;
    return t != null && (t.unstable_strictMode === !0 && (a = !0), t.identifierPrefix !== void 0 && (u = t.identifierPrefix), t.onUncaughtError !== void 0 && (e = t.onUncaughtError), t.onCaughtError !== void 0 && (n = t.onCaughtError), t.onRecoverableError !== void 0 && (c = t.onRecoverableError)), t = cd(
      l,
      1,
      !1,
      null,
      null,
      a,
      u,
      null,
      e,
      n,
      c,
      Sd
    ), l[Ka] = t.current, Zi(l), new ff(t);
  }, Te.hydrateRoot = function(l, t, a) {
    if (!L(l)) throw Error(d(299));
    var u = !1, e = "", n = Av, c = pv, i = Mv, f = null;
    return a != null && (a.unstable_strictMode === !0 && (u = !0), a.identifierPrefix !== void 0 && (e = a.identifierPrefix), a.onUncaughtError !== void 0 && (n = a.onUncaughtError), a.onCaughtError !== void 0 && (c = a.onCaughtError), a.onRecoverableError !== void 0 && (i = a.onRecoverableError), a.formState !== void 0 && (f = a.formState)), t = cd(
      l,
      1,
      !0,
      t,
      a ?? null,
      u,
      e,
      f,
      n,
      c,
      i,
      Sd
    ), t.context = id(null), a = t.current, u = gt(), u = Fn(u), e = sa(u), e.callback = null, va(a, e, u), a = u, t.current.lanes = a, Hu(t, a), xt(t), l[Ka] = t.current, Zi(l), new Zn(t);
  }, Te.version = "19.2.4", Te;
}
var Dd;
function vm() {
  if (Dd) return of.exports;
  Dd = 1;
  function _() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(_);
      } catch (D) {
        console.error(D);
      }
  }
  return _(), of.exports = sm(), of.exports;
}
var om = vm();
const dm = "_skeleton_xk662_19", ym = "_card_xk662_32", mm = "_row_xk662_40", hm = "_block_xk662_47", gm = "_textStack_xk662_54", Sm = "_textLine_xk662_61", rm = "_textLineLast_xk662_68", qt = {
  skeleton: dm,
  card: ym,
  row: mm,
  block: hm,
  textStack: gm,
  textLine: Sm,
  textLineLast: rm
};
function bm(_) {
  switch (_.variant) {
    case "card":
      return /* @__PURE__ */ p.jsx(
        "div",
        {
          className: [qt.skeleton, qt.card, _.className].filter(Boolean).join(" ")
        }
      );
    case "row":
      return /* @__PURE__ */ p.jsx(
        "div",
        {
          className: [qt.skeleton, qt.row, _.className].filter(Boolean).join(" ")
        }
      );
    case "block":
      return /* @__PURE__ */ p.jsx(
        "div",
        {
          className: [qt.skeleton, qt.block, _.className].filter(Boolean).join(" ")
        }
      );
    case "text": {
      const D = _.lines ?? 3;
      return /* @__PURE__ */ p.jsx(
        "div",
        {
          className: [qt.textStack, _.className].filter(Boolean).join(" "),
          children: Array.from({ length: D }, (U, d) => /* @__PURE__ */ p.jsx(
            "div",
            {
              className: [
                qt.skeleton,
                qt.textLine,
                d === D - 1 && D > 1 ? qt.textLineLast : ""
              ].filter(Boolean).join(" ")
            },
            d
          ))
        }
      );
    }
  }
}
const _m = "_badge_vkl6x_1", zm = "_ready_vkl6x_12", Em = "_planning_vkl6x_18", Tm = "_implementing_vkl6x_24", Am = "_reviewing_vkl6x_30", pm = "_verifying_vkl6x_36", Mm = "_done_vkl6x_42", Om = "_cancelled_vkl6x_48", Dm = "_pending_vkl6x_54", Um = "_running_vkl6x_60", Nm = "_complete_vkl6x_66", jm = "_failed_vkl6x_72", Hm = "_closed_vkl6x_78", Rm = "_blocked_vkl6x_84", xm = "_inReview_vkl6x_90", qm = "_loading_vkl6x_96", Bm = "_paused_vkl6x_102", Cm = "_unknown_vkl6x_108", Cl = {
  badge: _m,
  ready: zm,
  planning: Em,
  implementing: Tm,
  reviewing: Am,
  verifying: pm,
  done: Mm,
  cancelled: Om,
  pending: Dm,
  running: Um,
  complete: Nm,
  failed: jm,
  closed: Hm,
  blocked: Rm,
  inReview: xm,
  loading: qm,
  paused: Bm,
  unknown: Cm
}, Ym = {
  ready: "ready",
  planning: "planning",
  implementing: "implementing",
  reviewing: "reviewing",
  verifying: "verifying",
  done: "done",
  cancelled: "cancelled",
  pending: "pending",
  running: "running",
  complete: "complete",
  failed: "failed",
  closed: "closed",
  blocked: "blocked",
  in_review: "in review",
  loading: "loading",
  paused: "paused"
}, Gm = {
  ready: Cl.ready,
  planning: Cl.planning,
  implementing: Cl.implementing,
  reviewing: Cl.reviewing,
  verifying: Cl.verifying,
  done: Cl.done,
  cancelled: Cl.cancelled,
  pending: Cl.pending,
  running: Cl.running,
  complete: Cl.complete,
  failed: Cl.failed,
  closed: Cl.closed,
  blocked: Cl.blocked,
  in_review: Cl.inReview,
  loading: Cl.loading,
  paused: Cl.paused
};
function Ae({ status: _ }) {
  const D = Ym[_] ?? _, U = Gm[_] ?? Cl.unknown;
  return /* @__PURE__ */ p.jsx("span", { className: `${Cl.badge} ${U}`, children: D });
}
const Xm = "_root_1ahyv_1", Qm = "_dark_1ahyv_2", Zm = "_light_1ahyv_3", Lm = "_header_1ahyv_4", Vm = "_controls_1ahyv_4", Km = "_lifecycle_1ahyv_4", Jm = "_pipRail_1ahyv_4", wm = "_selectors_1ahyv_4", km = "_task_1ahyv_4", Wm = "_event_1ahyv_4", $m = "_connection_1ahyv_6", Fm = "_meta_1ahyv_6", Im = "_muted_1ahyv_6", Pm = "_stale_1ahyv_7", lh = "_chip_1ahyv_9", th = "_list_1ahyv_10", ah = "_card_1ahyv_11", uh = "_attention_1ahyv_11", eh = "_readiness_1ahyv_11", nh = "_waveBoard_1ahyv_13", ch = "_linkButton_1ahyv_25", ih = "_pip_1ahyv_4", fh = "_inlineMode_1ahyv_27", sh = "_fullscreen_1ahyv_27", cl = {
  root: Xm,
  dark: Qm,
  light: Zm,
  header: Lm,
  controls: Vm,
  lifecycle: Km,
  pipRail: Jm,
  selectors: wm,
  task: km,
  event: Wm,
  connection: $m,
  meta: Fm,
  muted: Im,
  stale: Pm,
  chip: lh,
  list: th,
  card: ah,
  attention: uh,
  readiness: eh,
  waveBoard: nh,
  linkButton: ch,
  pip: ih,
  inlineMode: fh,
  fullscreen: sh
};
function vh({ agents: _ }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "agents" }),
    /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "active agents", children: _.map((D, U) => /* @__PURE__ */ p.jsxs("li", { className: cl.card, children: [
      /* @__PURE__ */ p.jsxs("div", { children: [
        /* @__PURE__ */ p.jsx("strong", { children: D.role }),
        " · ",
        D.task,
        D.wave ? ` · wave ${D.wave}` : "",
        D.task_number ? ` task ${D.task_number}` : ""
      ] }),
      /* @__PURE__ */ p.jsxs("div", { className: cl.meta, children: [
        D.branch || "no branch",
        " · ",
        D.worktree || "no worktree",
        " · ",
        D.last_activity ? new Date(D.last_activity).toLocaleTimeString() : "no activity"
      ] }),
      /* @__PURE__ */ p.jsx(Ae, { status: D.paused ? "paused" : D.stage || (D.active ? "running" : "ready") })
    ] }, `${D.task}-${D.role}-${U}`)) })
  ] });
}
function oh({ items: _, action: D }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "attention" }),
    _.length === 0 ? /* @__PURE__ */ p.jsx("p", { className: cl.muted, children: "nothing needs attention" }) : /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "attention items", children: _.map((U, d) => /* @__PURE__ */ p.jsxs("li", { className: cl.attention, children: [
      /* @__PURE__ */ p.jsxs("div", { children: [
        /* @__PURE__ */ p.jsx("strong", { children: U.kind.replace(/_/g, " ") }),
        " · ",
        U.task
      ] }),
      U.detail && /* @__PURE__ */ p.jsx("p", { children: U.detail }),
      D && /* @__PURE__ */ p.jsx("button", { onClick: () => D(`look at the blocker on ${U.task}`), children: "look at blocker" })
    ] }, `${U.task}-${U.kind}-${d}`)) })
  ] });
}
function dh({ events: _ }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "events" }),
    /* @__PURE__ */ p.jsx("ol", { className: cl.list, "aria-label": "event feed", children: _.map((D, U) => /* @__PURE__ */ p.jsxs("li", { className: cl.event, children: [
      /* @__PURE__ */ p.jsx("time", { dateTime: D.at, children: new Date(D.at).toLocaleTimeString() }),
      /* @__PURE__ */ p.jsx("span", { children: D.message })
    ] }, `${D.at}-${U}`)) })
  ] });
}
function Ud({ lifecycle: _ }) {
  const D = ["planning", "ready", "implementing", "reviewing", "verifying"];
  return /* @__PURE__ */ p.jsx("div", { className: cl.lifecycle, role: "list", "aria-label": "lifecycle counts", children: D.map((U) => /* @__PURE__ */ p.jsxs("span", { role: "listitem", className: cl.chip, children: [
    /* @__PURE__ */ p.jsx("b", { children: _[U] }),
    " ",
    U
  ] }, U)) });
}
function yh() {
  return window.openai ?? {};
}
function mh() {
  const [_, D] = jl.useState(yh);
  return jl.useEffect(() => {
    const U = (d) => {
      const L = d.detail, yl = (L == null ? void 0 : L.globals) ?? L ?? {};
      D((ul) => ({ ...ul, ...yl }));
    };
    return window.addEventListener("openai:set_globals", U), () => window.removeEventListener("openai:set_globals", U);
  }, []), _;
}
function hf(_) {
  if (!_ || typeof _ != "object") return;
  const D = _, U = D.structuredContent ?? D.structured_content ?? _;
  if (!U || typeof U != "object") return;
  const d = U;
  if (!(d.schema_version !== 2 || typeof d.project != "string" || typeof d.daemon_running != "boolean" || !d.lifecycle || typeof d.lifecycle != "object" || !Array.isArray(d.active_agents) || !Array.isArray(d.attention) || !d.truncated || typeof d.truncated != "object"))
    return d;
}
function hh(_, D, U) {
  const [d, L] = jl.useState(() => hf(_.toolOutput)), [yl, ul] = jl.useState(!1), ml = `${D ?? ""}\0${U ?? ""}`, A = jl.useRef(ml);
  A.current = ml;
  const T = jl.useRef(!1), Z = jl.useRef(0), x = jl.useRef(void 0), ll = jl.useRef(!0), Hl = _.displayMode === "pip" || _.displayMode === "fullscreen" ? 2e3 : 3e3;
  jl.useEffect(() => {
    const El = hf(_.toolOutput);
    El && L(El);
  }, [_.toolOutput]), jl.useEffect(() => () => {
    ll.current = !1;
  }, []);
  const bl = jl.useCallback(async () => {
    var rl;
    const El = ml;
    if (!(T.current || document.visibilityState !== "visible" || !((rl = window.openai) != null && rl.callTool))) {
      T.current = !0;
      try {
        const Yl = hf(await window.openai.callTool("refresh_monitor", { project: D, task: U }));
        if (!Yl) throw new Error("refresh_monitor returned an invalid snapshot");
        ll.current && A.current === El && (L(Yl), Z.current = 0, ul(!1));
      } catch {
        ll.current && A.current === El && (Z.current += 1, ul(!0));
      } finally {
        T.current = !1;
      }
    }
  }, [D, ml, U]);
  return jl.useEffect(() => {
    let El = !1;
    const rl = () => {
      if (El) return;
      window.clearInterval(x.current);
      const Tl = Math.min(Hl * 2 ** Z.current, 3e4);
      x.current = window.setInterval(async () => {
        await bl(), rl();
      }, Tl);
    }, Yl = () => {
      document.visibilityState === "visible" ? (bl(), rl()) : window.clearInterval(x.current);
    };
    return rl(), document.addEventListener("visibilitychange", Yl), () => {
      El = !0, window.clearInterval(x.current), document.removeEventListener("visibilitychange", Yl);
    };
  }, [Hl, bl]), { snapshot: !D || (d == null ? void 0 : d.project) === D ? d : void 0, stale: yl, refresh: bl };
}
function gh({ focus: _, action: D }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "waves" }),
    /* @__PURE__ */ p.jsx("div", { className: cl.waveBoard, role: "list", children: _.waves.map((U) => /* @__PURE__ */ p.jsxs("article", { role: "listitem", className: cl.card, children: [
      /* @__PURE__ */ p.jsxs("header", { children: [
        /* @__PURE__ */ p.jsxs("strong", { children: [
          "wave ",
          U.wave
        ] }),
        U.active && /* @__PURE__ */ p.jsx("span", { children: " active" })
      ] }),
      /* @__PURE__ */ p.jsx("ul", { children: U.tasks.map((d) => /* @__PURE__ */ p.jsxs("li", { children: [
        /* @__PURE__ */ p.jsxs("span", { children: [
          d.number,
          ". ",
          d.title
        ] }),
        /* @__PURE__ */ p.jsx(Ae, { status: d.status })
      ] }, d.number)) }),
      U.active && D && /* @__PURE__ */ p.jsxs("button", { onClick: () => D(`start wave ${U.wave} on ${_.filename}`), children: [
        "start wave ",
        U.wave
      ] })
    ] }, U.wave)) })
  ] });
}
function Sh() {
  var Hl, bl, Rl, El, rl, Yl, Tl, kl, et, Gl, V, Xl, nt, Bt, ct, Ql, Nt;
  const _ = mh(), D = _.displayMode ?? "inline", U = ((Hl = _.widgetState) == null ? void 0 : Hl.project) ?? ((bl = _.toolInput) == null ? void 0 : bl.project) ?? ((Rl = _.toolOutput) == null ? void 0 : Rl.project) ?? ((rl = (El = _.toolOutput) == null ? void 0 : El.projects) == null ? void 0 : rl[0]), d = ((Yl = _.widgetState) == null ? void 0 : Yl.task) ?? ((Tl = _.toolInput) == null ? void 0 : Tl.task) ?? ((et = (kl = _.toolOutput) == null ? void 0 : kl.focus) == null ? void 0 : et.filename) ?? ((Xl = (V = (Gl = _.toolOutput) == null ? void 0 : Gl.tasks) == null ? void 0 : V[0]) == null ? void 0 : Xl.filename), [L, yl] = jl.useState(U), [ul, ml] = jl.useState(d), { snapshot: A, stale: T } = hh(_, L, ul);
  jl.useEffect(() => {
    var j;
    !L && A && yl(A.project || ((j = A.projects) == null ? void 0 : j[0]));
  }, [L, A]), jl.useEffect(() => {
    var j, tl, S;
    !ul && A && A.project === L && ml(((j = A.focus) == null ? void 0 : j.filename) ?? ((S = (tl = A.tasks) == null ? void 0 : tl[0]) == null ? void 0 : S.filename));
  }, [L, ul, A]), jl.useEffect(() => {
    var j, tl;
    (tl = (j = window.openai) == null ? void 0 : j.setWidgetState) == null || tl.call(j, { project: L, task: ul });
  }, [L, ul]);
  const Z = _.sendFollowUpMessage ? (j) => {
    var tl, S;
    (S = (tl = window.openai) == null ? void 0 : tl.sendFollowUpMessage) == null || S.call(tl, { prompt: j });
  } : void 0, x = (A == null ? void 0 : A.attention.length) ?? 0, ll = jl.useMemo(() => (A == null ? void 0 : A.active_agents.filter((j) => j.active && !j.paused).length) ?? 0, [A]);
  return A ? /* @__PURE__ */ p.jsxs("main", { className: `${cl.root} ${_.theme === "light" ? cl.light : cl.dark} ${D === "pip" ? cl.pip : D === "fullscreen" ? cl.fullscreen : cl.inlineMode}`, style: D === "inline" && _.maxHeight ? { maxHeight: _.maxHeight } : void 0, children: [
    /* @__PURE__ */ p.jsxs("header", { className: cl.header, children: [
      /* @__PURE__ */ p.jsxs("div", { children: [
        /* @__PURE__ */ p.jsx("strong", { children: "kasmos monitor" }),
        /* @__PURE__ */ p.jsx("span", { className: cl.connection, children: A.daemon_running ? "● live" : "○ daemon offline" })
      ] }),
      /* @__PURE__ */ p.jsxs("div", { className: cl.controls, children: [
        _.requestDisplayMode && D !== "pip" && /* @__PURE__ */ p.jsx("button", { "aria-label": "pin as picture in picture", onClick: () => {
          var j, tl;
          return void ((tl = (j = window.openai) == null ? void 0 : j.requestDisplayMode) == null ? void 0 : tl.call(j, { mode: "pip" }));
        }, children: "pin" }),
        _.requestDisplayMode && D !== "fullscreen" && /* @__PURE__ */ p.jsx("button", { "aria-label": "expand monitor", onClick: () => {
          var j, tl;
          return void ((tl = (j = window.openai) == null ? void 0 : j.requestDisplayMode) == null ? void 0 : tl.call(j, { mode: "fullscreen" }));
        }, children: "expand" })
      ] })
    ] }),
    /* @__PURE__ */ p.jsx("div", { className: cl.stale, "aria-live": "polite", children: T ? "stale · retrying with last known state" : "" }),
    D === "pip" ? /* @__PURE__ */ p.jsxs("div", { className: cl.pipRail, children: [
      /* @__PURE__ */ p.jsx(Ud, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ p.jsxs("span", { children: [
        ll,
        " running"
      ] }),
      /* @__PURE__ */ p.jsx(Ae, { status: x ? `${x} blocked` : "ready" })
    ] }) : /* @__PURE__ */ p.jsxs(p.Fragment, { children: [
      /* @__PURE__ */ p.jsx(Ud, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ p.jsxs("div", { className: cl.selectors, children: [
        (((nt = A.projects) == null ? void 0 : nt.length) ?? 0) > 1 && /* @__PURE__ */ p.jsxs("label", { children: [
          "project",
          /* @__PURE__ */ p.jsx("select", { value: L, onChange: (j) => {
            yl(j.target.value), ml(void 0);
          }, children: (Bt = A.projects) == null ? void 0 : Bt.map((j) => /* @__PURE__ */ p.jsx("option", { children: j }, j)) })
        ] }),
        (((ct = A.tasks) == null ? void 0 : ct.length) ?? 0) > 0 && /* @__PURE__ */ p.jsxs("label", { children: [
          "task",
          /* @__PURE__ */ p.jsx("select", { value: ul, onChange: (j) => ml(j.target.value), children: (Ql = A.tasks) == null ? void 0 : Ql.map((j) => /* @__PURE__ */ p.jsx("option", { value: j.filename, children: j.filename }, j.filename)) })
        ] })
      ] }),
      /* @__PURE__ */ p.jsxs("section", { children: [
        /* @__PURE__ */ p.jsx("h2", { children: "tasks" }),
        /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "tasks", children: (Nt = A.tasks) == null ? void 0 : Nt.map((j) => {
          const tl = j.subtasks_total ? Math.round(j.subtasks_done / j.subtasks_total * 100) : 0;
          return /* @__PURE__ */ p.jsxs("li", { className: cl.task, children: [
            /* @__PURE__ */ p.jsxs("div", { children: [
              /* @__PURE__ */ p.jsx("button", { className: cl.linkButton, onClick: () => ml(j.filename), children: j.filename }),
              /* @__PURE__ */ p.jsx(Ae, { status: j.blocked ? "blocked" : j.status })
            ] }),
            /* @__PURE__ */ p.jsx("progress", { value: j.subtasks_done, max: j.subtasks_total || 1, "aria-label": `${j.filename} progress` }),
            /* @__PURE__ */ p.jsxs("small", { children: [
              tl,
              "% · wave ",
              j.active_wave || 0,
              "/",
              j.total_waves || 0
            ] })
          ] }, j.filename);
        }) })
      ] }),
      /* @__PURE__ */ p.jsx(oh, { items: A.attention, action: Z }),
      D === "fullscreen" && /* @__PURE__ */ p.jsxs(p.Fragment, { children: [
        A.focus && /* @__PURE__ */ p.jsx(gh, { focus: A.focus, action: Z }),
        /* @__PURE__ */ p.jsx(vh, { agents: A.active_agents }),
        A.focus && /* @__PURE__ */ p.jsxs("section", { className: cl.readiness, children: [
          /* @__PURE__ */ p.jsx("h2", { children: "readiness" }),
          /* @__PURE__ */ p.jsx(Ae, { status: A.focus.readiness.status }),
          /* @__PURE__ */ p.jsxs("p", { children: [
            "review cycle ",
            A.focus.readiness.review_cycle ?? 0
          ] }),
          /* @__PURE__ */ p.jsxs("p", { children: [
            "checks: ",
            A.focus.readiness.pr_check_status || "not reported"
          ] }),
          /* @__PURE__ */ p.jsxs("p", { children: [
            "review: ",
            A.focus.readiness.pr_review_decision || "not reported"
          ] }),
          /* @__PURE__ */ p.jsxs("p", { children: [
            "verification: ",
            A.focus.readiness.last_verify_outcome || "not reported"
          ] }),
          A.focus.readiness.has_review_feedback && Z && /* @__PURE__ */ p.jsx("button", { onClick: () => {
            var j;
            return Z(`approve review for ${(j = A.focus) == null ? void 0 : j.filename}`);
          }, children: "approve review" })
        ] }),
        /* @__PURE__ */ p.jsx(dh, { events: A.events ?? [] })
      ] })
    ] })
  ] }) : /* @__PURE__ */ p.jsx("main", { className: cl.root, children: /* @__PURE__ */ p.jsx(bm, { variant: "text", lines: 4 }) });
}
const Nd = document.getElementById("root");
Nd && om.createRoot(Nd).render(/* @__PURE__ */ p.jsx(em.StrictMode, { children: /* @__PURE__ */ p.jsx(Sh, {}) }));
