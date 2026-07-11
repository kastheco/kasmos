function Py(_) {
  return _ && _.__esModule && Object.prototype.hasOwnProperty.call(_, "default") ? _.default : _;
}
var sf = { exports: {} }, Te = {};
/**
 * @license React
 * react-jsx-runtime.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Sd;
function lm() {
  if (Sd) return Te;
  Sd = 1;
  var _ = Symbol.for("react.transitional.element"), D = Symbol.for("react.fragment");
  function H(m, w, yl) {
    var ul = null;
    if (yl !== void 0 && (ul = "" + yl), w.key !== void 0 && (ul = "" + w.key), "key" in w) {
      yl = {};
      for (var gl in w)
        gl !== "key" && (yl[gl] = w[gl]);
    } else yl = w;
    return w = yl.ref, {
      $$typeof: _,
      type: m,
      key: ul,
      ref: w !== void 0 ? w : null,
      props: yl
    };
  }
  return Te.Fragment = D, Te.jsx = H, Te.jsxs = H, Te;
}
var rd;
function tm() {
  return rd || (rd = 1, sf.exports = lm()), sf.exports;
}
var p = tm(), vf = { exports: {} }, Y = {};
/**
 * @license React
 * react.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var bd;
function am() {
  if (bd) return Y;
  bd = 1;
  var _ = Symbol.for("react.transitional.element"), D = Symbol.for("react.portal"), H = Symbol.for("react.fragment"), m = Symbol.for("react.strict_mode"), w = Symbol.for("react.profiler"), yl = Symbol.for("react.consumer"), ul = Symbol.for("react.context"), gl = Symbol.for("react.forward_ref"), A = Symbol.for("react.suspense"), E = Symbol.for("react.memo"), Q = Symbol.for("react.lazy"), q = Symbol.for("react.activity"), P = Symbol.iterator;
  function rl(v) {
    return v === null || typeof v != "object" ? null : (v = P && v[P] || v["@@iterator"], typeof v == "function" ? v : null);
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
  }, pl = Object.assign, ht = {};
  function Vl(v, T, O) {
    this.props = v, this.context = T, this.refs = ht, this.updater = O || bl;
  }
  Vl.prototype.isReactComponent = {}, Vl.prototype.setState = function(v, T) {
    if (typeof v != "object" && typeof v != "function" && v != null)
      throw Error(
        "takes an object of state variables to update or a function which returns an object of state variables."
      );
    this.updater.enqueueSetState(this, v, T, "setState");
  }, Vl.prototype.forceUpdate = function(v) {
    this.updater.enqueueForceUpdate(this, v, "forceUpdate");
  };
  function Mt() {
  }
  Mt.prototype = Vl.prototype;
  function Nl(v, T, O) {
    this.props = v, this.context = T, this.refs = ht, this.updater = O || bl;
  }
  var Jl = Nl.prototype = new Mt();
  Jl.constructor = Nl, pl(Jl, Vl.prototype), Jl.isPureReactComponent = !0;
  var at = Array.isArray;
  function Bl() {
  }
  var L = { H: null, A: null, T: null, S: null }, Cl = Object.prototype.hasOwnProperty;
  function ut(v, T, O) {
    var j = O.ref;
    return {
      $$typeof: _,
      type: v,
      key: T,
      ref: j !== void 0 ? j : null,
      props: O
    };
  }
  function Bt(v, T) {
    return ut(v.type, T, v.props);
  }
  function et(v) {
    return typeof v == "object" && v !== null && v.$$typeof === _;
  }
  function Yl(v) {
    var T = { "=": "=0", ":": "=2" };
    return "$" + v.replace(/[=:]/g, function(O) {
      return T[O];
    });
  }
  var Nt = /\/+/g;
  function N(v, T) {
    return typeof v == "object" && v !== null && v.key != null ? Yl("" + v.key) : T.toString(36);
  }
  function tl(v) {
    switch (v.status) {
      case "fulfilled":
        return v.value;
      case "rejected":
        throw v.reason;
      default:
        switch (typeof v.status == "string" ? v.then(Bl, Bl) : (v.status = "pending", v.then(
          function(T) {
            v.status === "pending" && (v.status = "fulfilled", v.value = T);
          },
          function(T) {
            v.status === "pending" && (v.status = "rejected", v.reason = T);
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
  function S(v, T, O, j, G) {
    var V = typeof v;
    (V === "undefined" || V === "boolean") && (v = null);
    var al = !1;
    if (v === null) al = !0;
    else
      switch (V) {
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
            case Q:
              return al = v._init, S(
                al(v._payload),
                T,
                O,
                j,
                G
              );
          }
      }
    if (al)
      return G = G(v), al = j === "" ? "." + N(v, 0) : j, at(G) ? (O = "", al != null && (O = al.replace(Nt, "$&/") + "/"), S(G, T, O, "", function(Uu) {
        return Uu;
      })) : G != null && (et(G) && (G = Bt(
        G,
        O + (G.key == null || v && v.key === G.key ? "" : ("" + G.key).replace(
          Nt,
          "$&/"
        ) + "/") + al
      )), T.push(G)), 1;
    al = 0;
    var wl = j === "" ? "." : j + ":";
    if (at(v))
      for (var Tl = 0; Tl < v.length; Tl++)
        j = v[Tl], V = wl + N(j, Tl), al += S(
          j,
          T,
          O,
          V,
          G
        );
    else if (Tl = rl(v), typeof Tl == "function")
      for (v = Tl.call(v), Tl = 0; !(j = v.next()).done; )
        j = j.value, V = wl + N(j, Tl++), al += S(
          j,
          T,
          O,
          V,
          G
        );
    else if (V === "object") {
      if (typeof v.then == "function")
        return S(
          tl(v),
          T,
          O,
          j,
          G
        );
      throw T = String(v), Error(
        "Objects are not valid as a React child (found: " + (T === "[object Object]" ? "object with keys {" + Object.keys(v).join(", ") + "}" : T) + "). If you meant to render a collection of children, use an array instead."
      );
    }
    return al;
  }
  function M(v, T, O) {
    if (v == null) return v;
    var j = [], G = 0;
    return S(v, j, "", "", function(V) {
      return T.call(O, V, G++);
    }), j;
  }
  function C(v) {
    if (v._status === -1) {
      var T = v._result;
      T = T(), T.then(
        function(O) {
          (v._status === 0 || v._status === -1) && (v._status = 1, v._result = O);
        },
        function(O) {
          (v._status === 0 || v._status === -1) && (v._status = 2, v._result = O);
        }
      ), v._status === -1 && (v._status = 0, v._result = T);
    }
    if (v._status === 1) return v._result.default;
    throw v._result;
  }
  var il = typeof reportError == "function" ? reportError : function(v) {
    if (typeof window == "object" && typeof window.ErrorEvent == "function") {
      var T = new window.ErrorEvent("error", {
        bubbles: !0,
        cancelable: !0,
        message: typeof v == "object" && v !== null && typeof v.message == "string" ? String(v.message) : String(v),
        error: v
      });
      if (!window.dispatchEvent(T)) return;
    } else if (typeof process == "object" && typeof process.emit == "function") {
      process.emit("uncaughtException", v);
      return;
    }
    console.error(v);
  }, dl = {
    map: M,
    forEach: function(v, T, O) {
      M(
        v,
        function() {
          T.apply(this, arguments);
        },
        O
      );
    },
    count: function(v) {
      var T = 0;
      return M(v, function() {
        T++;
      }), T;
    },
    toArray: function(v) {
      return M(v, function(T) {
        return T;
      }) || [];
    },
    only: function(v) {
      if (!et(v))
        throw Error(
          "React.Children.only expected to receive a single React element child."
        );
      return v;
    }
  };
  return Y.Activity = q, Y.Children = dl, Y.Component = Vl, Y.Fragment = H, Y.Profiler = w, Y.PureComponent = Nl, Y.StrictMode = m, Y.Suspense = A, Y.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = L, Y.__COMPILER_RUNTIME = {
    __proto__: null,
    c: function(v) {
      return L.H.useMemoCache(v);
    }
  }, Y.cache = function(v) {
    return function() {
      return v.apply(null, arguments);
    };
  }, Y.cacheSignal = function() {
    return null;
  }, Y.cloneElement = function(v, T, O) {
    if (v == null)
      throw Error(
        "The argument must be a React element, but you passed " + v + "."
      );
    var j = pl({}, v.props), G = v.key;
    if (T != null)
      for (V in T.key !== void 0 && (G = "" + T.key), T)
        !Cl.call(T, V) || V === "key" || V === "__self" || V === "__source" || V === "ref" && T.ref === void 0 || (j[V] = T[V]);
    var V = arguments.length - 2;
    if (V === 1) j.children = O;
    else if (1 < V) {
      for (var al = Array(V), wl = 0; wl < V; wl++)
        al[wl] = arguments[wl + 2];
      j.children = al;
    }
    return ut(v.type, G, j);
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
  }, Y.createElement = function(v, T, O) {
    var j, G = {}, V = null;
    if (T != null)
      for (j in T.key !== void 0 && (V = "" + T.key), T)
        Cl.call(T, j) && j !== "key" && j !== "__self" && j !== "__source" && (G[j] = T[j]);
    var al = arguments.length - 2;
    if (al === 1) G.children = O;
    else if (1 < al) {
      for (var wl = Array(al), Tl = 0; Tl < al; Tl++)
        wl[Tl] = arguments[Tl + 2];
      G.children = wl;
    }
    if (v && v.defaultProps)
      for (j in al = v.defaultProps, al)
        G[j] === void 0 && (G[j] = al[j]);
    return ut(v, V, G);
  }, Y.createRef = function() {
    return { current: null };
  }, Y.forwardRef = function(v) {
    return { $$typeof: gl, render: v };
  }, Y.isValidElement = et, Y.lazy = function(v) {
    return {
      $$typeof: Q,
      _payload: { _status: -1, _result: v },
      _init: C
    };
  }, Y.memo = function(v, T) {
    return {
      $$typeof: E,
      type: v,
      compare: T === void 0 ? null : T
    };
  }, Y.startTransition = function(v) {
    var T = L.T, O = {};
    L.T = O;
    try {
      var j = v(), G = L.S;
      G !== null && G(O, j), typeof j == "object" && j !== null && typeof j.then == "function" && j.then(Bl, il);
    } catch (V) {
      il(V);
    } finally {
      T !== null && O.types !== null && (T.types = O.types), L.T = T;
    }
  }, Y.unstable_useCacheRefresh = function() {
    return L.H.useCacheRefresh();
  }, Y.use = function(v) {
    return L.H.use(v);
  }, Y.useActionState = function(v, T, O) {
    return L.H.useActionState(v, T, O);
  }, Y.useCallback = function(v, T) {
    return L.H.useCallback(v, T);
  }, Y.useContext = function(v) {
    return L.H.useContext(v);
  }, Y.useDebugValue = function() {
  }, Y.useDeferredValue = function(v, T) {
    return L.H.useDeferredValue(v, T);
  }, Y.useEffect = function(v, T) {
    return L.H.useEffect(v, T);
  }, Y.useEffectEvent = function(v) {
    return L.H.useEffectEvent(v);
  }, Y.useId = function() {
    return L.H.useId();
  }, Y.useImperativeHandle = function(v, T, O) {
    return L.H.useImperativeHandle(v, T, O);
  }, Y.useInsertionEffect = function(v, T) {
    return L.H.useInsertionEffect(v, T);
  }, Y.useLayoutEffect = function(v, T) {
    return L.H.useLayoutEffect(v, T);
  }, Y.useMemo = function(v, T) {
    return L.H.useMemo(v, T);
  }, Y.useOptimistic = function(v, T) {
    return L.H.useOptimistic(v, T);
  }, Y.useReducer = function(v, T, O) {
    return L.H.useReducer(v, T, O);
  }, Y.useRef = function(v) {
    return L.H.useRef(v);
  }, Y.useState = function(v) {
    return L.H.useState(v);
  }, Y.useSyncExternalStore = function(v, T, O) {
    return L.H.useSyncExternalStore(
      v,
      T,
      O
    );
  }, Y.useTransition = function() {
    return L.H.useTransition();
  }, Y.version = "19.2.4", Y;
}
var _d;
function hf() {
  return _d || (_d = 1, vf.exports = am()), vf.exports;
}
var ql = hf();
const um = /* @__PURE__ */ Py(ql);
var df = { exports: {} }, Ee = {}, of = { exports: {} }, yf = {};
/**
 * @license React
 * scheduler.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var zd;
function em() {
  return zd || (zd = 1, (function(_) {
    function D(S, M) {
      var C = S.length;
      S.push(M);
      l: for (; 0 < C; ) {
        var il = C - 1 >>> 1, dl = S[il];
        if (0 < w(dl, M))
          S[il] = M, S[C] = dl, C = il;
        else break l;
      }
    }
    function H(S) {
      return S.length === 0 ? null : S[0];
    }
    function m(S) {
      if (S.length === 0) return null;
      var M = S[0], C = S.pop();
      if (C !== M) {
        S[0] = C;
        l: for (var il = 0, dl = S.length, v = dl >>> 1; il < v; ) {
          var T = 2 * (il + 1) - 1, O = S[T], j = T + 1, G = S[j];
          if (0 > w(O, C))
            j < dl && 0 > w(G, O) ? (S[il] = G, S[j] = C, il = j) : (S[il] = O, S[T] = C, il = T);
          else if (j < dl && 0 > w(G, C))
            S[il] = G, S[j] = C, il = j;
          else break l;
        }
      }
      return M;
    }
    function w(S, M) {
      var C = S.sortIndex - M.sortIndex;
      return C !== 0 ? C : S.id - M.id;
    }
    if (_.unstable_now = void 0, typeof performance == "object" && typeof performance.now == "function") {
      var yl = performance;
      _.unstable_now = function() {
        return yl.now();
      };
    } else {
      var ul = Date, gl = ul.now();
      _.unstable_now = function() {
        return ul.now() - gl;
      };
    }
    var A = [], E = [], Q = 1, q = null, P = 3, rl = !1, bl = !1, pl = !1, ht = !1, Vl = typeof setTimeout == "function" ? setTimeout : null, Mt = typeof clearTimeout == "function" ? clearTimeout : null, Nl = typeof setImmediate < "u" ? setImmediate : null;
    function Jl(S) {
      for (var M = H(E); M !== null; ) {
        if (M.callback === null) m(E);
        else if (M.startTime <= S)
          m(E), M.sortIndex = M.expirationTime, D(A, M);
        else break;
        M = H(E);
      }
    }
    function at(S) {
      if (pl = !1, Jl(S), !bl)
        if (H(A) !== null)
          bl = !0, Bl || (Bl = !0, Yl());
        else {
          var M = H(E);
          M !== null && tl(at, M.startTime - S);
        }
    }
    var Bl = !1, L = -1, Cl = 5, ut = -1;
    function Bt() {
      return ht ? !0 : !(_.unstable_now() - ut < Cl);
    }
    function et() {
      if (ht = !1, Bl) {
        var S = _.unstable_now();
        ut = S;
        var M = !0;
        try {
          l: {
            bl = !1, pl && (pl = !1, Mt(L), L = -1), rl = !0;
            var C = P;
            try {
              t: {
                for (Jl(S), q = H(A); q !== null && !(q.expirationTime > S && Bt()); ) {
                  var il = q.callback;
                  if (typeof il == "function") {
                    q.callback = null, P = q.priorityLevel;
                    var dl = il(
                      q.expirationTime <= S
                    );
                    if (S = _.unstable_now(), typeof dl == "function") {
                      q.callback = dl, Jl(S), M = !0;
                      break t;
                    }
                    q === H(A) && m(A), Jl(S);
                  } else m(A);
                  q = H(A);
                }
                if (q !== null) M = !0;
                else {
                  var v = H(E);
                  v !== null && tl(
                    at,
                    v.startTime - S
                  ), M = !1;
                }
              }
              break l;
            } finally {
              q = null, P = C, rl = !1;
            }
            M = void 0;
          }
        } finally {
          M ? Yl() : Bl = !1;
        }
      }
    }
    var Yl;
    if (typeof Nl == "function")
      Yl = function() {
        Nl(et);
      };
    else if (typeof MessageChannel < "u") {
      var Nt = new MessageChannel(), N = Nt.port2;
      Nt.port1.onmessage = et, Yl = function() {
        N.postMessage(null);
      };
    } else
      Yl = function() {
        Vl(et, 0);
      };
    function tl(S, M) {
      L = Vl(function() {
        S(_.unstable_now());
      }, M);
    }
    _.unstable_IdlePriority = 5, _.unstable_ImmediatePriority = 1, _.unstable_LowPriority = 4, _.unstable_NormalPriority = 3, _.unstable_Profiling = null, _.unstable_UserBlockingPriority = 2, _.unstable_cancelCallback = function(S) {
      S.callback = null;
    }, _.unstable_forceFrameRate = function(S) {
      0 > S || 125 < S ? console.error(
        "forceFrameRate takes a positive int between 0 and 125, forcing frame rates higher than 125 fps is not supported"
      ) : Cl = 0 < S ? Math.floor(1e3 / S) : 5;
    }, _.unstable_getCurrentPriorityLevel = function() {
      return P;
    }, _.unstable_next = function(S) {
      switch (P) {
        case 1:
        case 2:
        case 3:
          var M = 3;
          break;
        default:
          M = P;
      }
      var C = P;
      P = M;
      try {
        return S();
      } finally {
        P = C;
      }
    }, _.unstable_requestPaint = function() {
      ht = !0;
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
      var C = P;
      P = S;
      try {
        return M();
      } finally {
        P = C;
      }
    }, _.unstable_scheduleCallback = function(S, M, C) {
      var il = _.unstable_now();
      switch (typeof C == "object" && C !== null ? (C = C.delay, C = typeof C == "number" && 0 < C ? il + C : il) : C = il, S) {
        case 1:
          var dl = -1;
          break;
        case 2:
          dl = 250;
          break;
        case 5:
          dl = 1073741823;
          break;
        case 4:
          dl = 1e4;
          break;
        default:
          dl = 5e3;
      }
      return dl = C + dl, S = {
        id: Q++,
        callback: M,
        priorityLevel: S,
        startTime: C,
        expirationTime: dl,
        sortIndex: -1
      }, C > il ? (S.sortIndex = C, D(E, S), H(A) === null && S === H(E) && (pl ? (Mt(L), L = -1) : pl = !0, tl(at, C - il))) : (S.sortIndex = dl, D(A, S), bl || rl || (bl = !0, Bl || (Bl = !0, Yl()))), S;
    }, _.unstable_shouldYield = Bt, _.unstable_wrapCallback = function(S) {
      var M = P;
      return function() {
        var C = P;
        P = M;
        try {
          return S.apply(this, arguments);
        } finally {
          P = C;
        }
      };
    };
  })(yf)), yf;
}
var Td;
function nm() {
  return Td || (Td = 1, of.exports = em()), of.exports;
}
var mf = { exports: {} }, Kl = {};
/**
 * @license React
 * react-dom.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Ed;
function cm() {
  if (Ed) return Kl;
  Ed = 1;
  var _ = hf();
  function D(A) {
    var E = "https://react.dev/errors/" + A;
    if (1 < arguments.length) {
      E += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var Q = 2; Q < arguments.length; Q++)
        E += "&args[]=" + encodeURIComponent(arguments[Q]);
    }
    return "Minified React error #" + A + "; visit " + E + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function H() {
  }
  var m = {
    d: {
      f: H,
      r: function() {
        throw Error(D(522));
      },
      D: H,
      C: H,
      L: H,
      m: H,
      X: H,
      S: H,
      M: H
    },
    p: 0,
    findDOMNode: null
  }, w = Symbol.for("react.portal");
  function yl(A, E, Q) {
    var q = 3 < arguments.length && arguments[3] !== void 0 ? arguments[3] : null;
    return {
      $$typeof: w,
      key: q == null ? null : "" + q,
      children: A,
      containerInfo: E,
      implementation: Q
    };
  }
  var ul = _.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE;
  function gl(A, E) {
    if (A === "font") return "";
    if (typeof E == "string")
      return E === "use-credentials" ? E : "";
  }
  return Kl.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = m, Kl.createPortal = function(A, E) {
    var Q = 2 < arguments.length && arguments[2] !== void 0 ? arguments[2] : null;
    if (!E || E.nodeType !== 1 && E.nodeType !== 9 && E.nodeType !== 11)
      throw Error(D(299));
    return yl(A, E, null, Q);
  }, Kl.flushSync = function(A) {
    var E = ul.T, Q = m.p;
    try {
      if (ul.T = null, m.p = 2, A) return A();
    } finally {
      ul.T = E, m.p = Q, m.d.f();
    }
  }, Kl.preconnect = function(A, E) {
    typeof A == "string" && (E ? (E = E.crossOrigin, E = typeof E == "string" ? E === "use-credentials" ? E : "" : void 0) : E = null, m.d.C(A, E));
  }, Kl.prefetchDNS = function(A) {
    typeof A == "string" && m.d.D(A);
  }, Kl.preinit = function(A, E) {
    if (typeof A == "string" && E && typeof E.as == "string") {
      var Q = E.as, q = gl(Q, E.crossOrigin), P = typeof E.integrity == "string" ? E.integrity : void 0, rl = typeof E.fetchPriority == "string" ? E.fetchPriority : void 0;
      Q === "style" ? m.d.S(
        A,
        typeof E.precedence == "string" ? E.precedence : void 0,
        {
          crossOrigin: q,
          integrity: P,
          fetchPriority: rl
        }
      ) : Q === "script" && m.d.X(A, {
        crossOrigin: q,
        integrity: P,
        fetchPriority: rl,
        nonce: typeof E.nonce == "string" ? E.nonce : void 0
      });
    }
  }, Kl.preinitModule = function(A, E) {
    if (typeof A == "string")
      if (typeof E == "object" && E !== null) {
        if (E.as == null || E.as === "script") {
          var Q = gl(
            E.as,
            E.crossOrigin
          );
          m.d.M(A, {
            crossOrigin: Q,
            integrity: typeof E.integrity == "string" ? E.integrity : void 0,
            nonce: typeof E.nonce == "string" ? E.nonce : void 0
          });
        }
      } else E == null && m.d.M(A);
  }, Kl.preload = function(A, E) {
    if (typeof A == "string" && typeof E == "object" && E !== null && typeof E.as == "string") {
      var Q = E.as, q = gl(Q, E.crossOrigin);
      m.d.L(A, Q, {
        crossOrigin: q,
        integrity: typeof E.integrity == "string" ? E.integrity : void 0,
        nonce: typeof E.nonce == "string" ? E.nonce : void 0,
        type: typeof E.type == "string" ? E.type : void 0,
        fetchPriority: typeof E.fetchPriority == "string" ? E.fetchPriority : void 0,
        referrerPolicy: typeof E.referrerPolicy == "string" ? E.referrerPolicy : void 0,
        imageSrcSet: typeof E.imageSrcSet == "string" ? E.imageSrcSet : void 0,
        imageSizes: typeof E.imageSizes == "string" ? E.imageSizes : void 0,
        media: typeof E.media == "string" ? E.media : void 0
      });
    }
  }, Kl.preloadModule = function(A, E) {
    if (typeof A == "string")
      if (E) {
        var Q = gl(E.as, E.crossOrigin);
        m.d.m(A, {
          as: typeof E.as == "string" && E.as !== "script" ? E.as : void 0,
          crossOrigin: Q,
          integrity: typeof E.integrity == "string" ? E.integrity : void 0
        });
      } else m.d.m(A);
  }, Kl.requestFormReset = function(A) {
    m.d.r(A);
  }, Kl.unstable_batchedUpdates = function(A, E) {
    return A(E);
  }, Kl.useFormState = function(A, E, Q) {
    return ul.H.useFormState(A, E, Q);
  }, Kl.useFormStatus = function() {
    return ul.H.useHostTransitionStatus();
  }, Kl.version = "19.2.4", Kl;
}
var Ad;
function im() {
  if (Ad) return mf.exports;
  Ad = 1;
  function _() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(_);
      } catch (D) {
        console.error(D);
      }
  }
  return _(), mf.exports = cm(), mf.exports;
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
var pd;
function fm() {
  if (pd) return Ee;
  pd = 1;
  var _ = nm(), D = hf(), H = im();
  function m(l) {
    var t = "https://react.dev/errors/" + l;
    if (1 < arguments.length) {
      t += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var a = 2; a < arguments.length; a++)
        t += "&args[]=" + encodeURIComponent(arguments[a]);
    }
    return "Minified React error #" + l + "; visit " + t + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function w(l) {
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
  function gl(l) {
    if (l.tag === 31) {
      var t = l.memoizedState;
      if (t === null && (l = l.alternate, l !== null && (t = l.memoizedState)), t !== null) return t.dehydrated;
    }
    return null;
  }
  function A(l) {
    if (yl(l) !== l)
      throw Error(m(188));
  }
  function E(l) {
    var t = l.alternate;
    if (!t) {
      if (t = yl(l), t === null) throw Error(m(188));
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
        throw Error(m(188));
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
          if (!c) throw Error(m(189));
        }
      }
      if (a.alternate !== u) throw Error(m(190));
    }
    if (a.tag !== 3) throw Error(m(188));
    return a.stateNode.current === a ? l : t;
  }
  function Q(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l;
    for (l = l.child; l !== null; ) {
      if (t = Q(l), t !== null) return t;
      l = l.sibling;
    }
    return null;
  }
  var q = Object.assign, P = Symbol.for("react.element"), rl = Symbol.for("react.transitional.element"), bl = Symbol.for("react.portal"), pl = Symbol.for("react.fragment"), ht = Symbol.for("react.strict_mode"), Vl = Symbol.for("react.profiler"), Mt = Symbol.for("react.consumer"), Nl = Symbol.for("react.context"), Jl = Symbol.for("react.forward_ref"), at = Symbol.for("react.suspense"), Bl = Symbol.for("react.suspense_list"), L = Symbol.for("react.memo"), Cl = Symbol.for("react.lazy"), ut = Symbol.for("react.activity"), Bt = Symbol.for("react.memo_cache_sentinel"), et = Symbol.iterator;
  function Yl(l) {
    return l === null || typeof l != "object" ? null : (l = et && l[et] || l["@@iterator"], typeof l == "function" ? l : null);
  }
  var Nt = Symbol.for("react.client.reference");
  function N(l) {
    if (l == null) return null;
    if (typeof l == "function")
      return l.$$typeof === Nt ? null : l.displayName || l.name || null;
    if (typeof l == "string") return l;
    switch (l) {
      case pl:
        return "Fragment";
      case Vl:
        return "Profiler";
      case ht:
        return "StrictMode";
      case at:
        return "Suspense";
      case Bl:
        return "SuspenseList";
      case ut:
        return "Activity";
    }
    if (typeof l == "object")
      switch (l.$$typeof) {
        case bl:
          return "Portal";
        case Nl:
          return l.displayName || "Context";
        case Mt:
          return (l._context.displayName || "Context") + ".Consumer";
        case Jl:
          var t = l.render;
          return l = l.displayName, l || (l = t.displayName || t.name || "", l = l !== "" ? "ForwardRef(" + l + ")" : "ForwardRef"), l;
        case L:
          return t = l.displayName || null, t !== null ? t : N(l.type) || "Memo";
        case Cl:
          t = l._payload, l = l._init;
          try {
            return N(l(t));
          } catch {
          }
      }
    return null;
  }
  var tl = Array.isArray, S = D.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, M = H.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, C = {
    pending: !1,
    data: null,
    method: null,
    action: null
  }, il = [], dl = -1;
  function v(l) {
    return { current: l };
  }
  function T(l) {
    0 > dl || (l.current = il[dl], il[dl] = null, dl--);
  }
  function O(l, t) {
    dl++, il[dl] = l.current, l.current = t;
  }
  var j = v(null), G = v(null), V = v(null), al = v(null);
  function wl(l, t) {
    switch (O(V, t), O(G, l), O(j, null), t.nodeType) {
      case 9:
      case 11:
        l = (l = t.documentElement) && (l = l.namespaceURI) ? X0(l) : 0;
        break;
      default:
        if (l = t.tagName, t = t.namespaceURI)
          t = X0(t), l = Q0(t, l);
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
    T(j), O(j, l);
  }
  function Tl() {
    T(j), T(G), T(V);
  }
  function Uu(l) {
    l.memoizedState !== null && O(al, l);
    var t = j.current, a = Q0(t, l.type);
    t !== a && (O(G, l), O(j, a));
  }
  function pe(l) {
    G.current === l && (T(j), T(G)), al.current === l && (T(al), re._currentValue = C);
  }
  var Vn, gf;
  function Ma(l) {
    if (Vn === void 0)
      try {
        throw Error();
      } catch (a) {
        var t = a.stack.trim().match(/\n( *(at )?)/);
        Vn = t && t[1] || "", gf = -1 < a.stack.indexOf(`
    at`) ? " (<anonymous>)" : -1 < a.stack.indexOf("@") ? "@unknown:0:0" : "";
      }
    return `
` + Vn + l + gf;
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
`), y = i.split(`
`);
        for (e = u = 0; u < f.length && !f[u].includes("DetermineComponentFrameRoot"); )
          u++;
        for (; e < y.length && !y[e].includes(
          "DetermineComponentFrameRoot"
        ); )
          e++;
        if (u === f.length || e === y.length)
          for (u = f.length - 1, e = y.length - 1; 1 <= u && 0 <= e && f[u] !== y[e]; )
            e--;
        for (; 1 <= u && 0 <= e; u--, e--)
          if (f[u] !== y[e]) {
            if (u !== 1 || e !== 1)
              do
                if (u--, e--, 0 > e || f[u] !== y[e]) {
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
  function Ud(l, t) {
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
  function Sf(l) {
    try {
      var t = "", a = null;
      do
        t += Ud(l, a), a = l, l = l.return;
      while (l);
      return t;
    } catch (u) {
      return `
Error generating stack: ` + u.message + `
` + u.stack;
    }
  }
  var wn = Object.prototype.hasOwnProperty, kn = _.unstable_scheduleCallback, Wn = _.unstable_cancelCallback, Nd = _.unstable_shouldYield, jd = _.unstable_requestPaint, nt = _.unstable_now, Hd = _.unstable_getCurrentPriorityLevel, rf = _.unstable_ImmediatePriority, bf = _.unstable_UserBlockingPriority, Me = _.unstable_NormalPriority, Rd = _.unstable_LowPriority, _f = _.unstable_IdlePriority, xd = _.log, qd = _.unstable_setDisableYieldValue, Nu = null, ct = null;
  function ta(l) {
    if (typeof xd == "function" && qd(l), ct && typeof ct.setStrictMode == "function")
      try {
        ct.setStrictMode(Nu, l);
      } catch {
      }
  }
  var it = Math.clz32 ? Math.clz32 : Yd, Bd = Math.log, Cd = Math.LN2;
  function Yd(l) {
    return l >>>= 0, l === 0 ? 32 : 31 - (Bd(l) / Cd | 0) | 0;
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
  function Gd(l, t) {
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
  function zf() {
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
  function Xd(l, t, a, u, e, n) {
    var c = l.pendingLanes;
    l.pendingLanes = a, l.suspendedLanes = 0, l.pingedLanes = 0, l.warmLanes = 0, l.expiredLanes &= a, l.entangledLanes &= a, l.errorRecoveryDisabledLanes &= a, l.shellSuspendCounter = 0;
    var i = l.entanglements, f = l.expirationTimes, y = l.hiddenUpdates;
    for (a = c & ~a; 0 < a; ) {
      var r = 31 - it(a), z = 1 << r;
      i[r] = 0, f[r] = -1;
      var h = y[r];
      if (h !== null)
        for (y[r] = null, r = 0; r < h.length; r++) {
          var g = h[r];
          g !== null && (g.lane &= -536870913);
        }
      a &= ~z;
    }
    u !== 0 && Tf(l, u, 0), n !== 0 && e === 0 && l.tag !== 0 && (l.suspendedLanes |= n & ~(c & ~t));
  }
  function Tf(l, t, a) {
    l.pendingLanes |= t, l.suspendedLanes &= ~t;
    var u = 31 - it(t);
    l.entangledLanes |= t, l.entanglements[u] = l.entanglements[u] | 1073741824 | a & 261930;
  }
  function Ef(l, t) {
    var a = l.entangledLanes |= t;
    for (l = l.entanglements; a; ) {
      var u = 31 - it(a), e = 1 << u;
      e & t | l[u] & t && (l[u] |= t), a &= ~e;
    }
  }
  function Af(l, t) {
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
  function pf() {
    var l = M.p;
    return l !== 0 ? l : (l = window.event, l === void 0 ? 32 : vd(l.type));
  }
  function Mf(l, t) {
    var a = M.p;
    try {
      return M.p = l, t();
    } finally {
      M.p = a;
    }
  }
  var aa = Math.random().toString(36).slice(2), Gl = "__reactFiber$" + aa, Wl = "__reactProps$" + aa, Ka = "__reactContainer$" + aa, Pn = "__reactEvents$" + aa, Qd = "__reactListeners$" + aa, Zd = "__reactHandles$" + aa, Of = "__reactResources$" + aa, Ru = "__reactMarker$" + aa;
  function lc(l) {
    delete l[Gl], delete l[Wl], delete l[Pn], delete l[Qd], delete l[Zd];
  }
  function Ja(l) {
    var t = l[Gl];
    if (t) return t;
    for (var a = l.parentNode; a; ) {
      if (t = a[Ka] || a[Gl]) {
        if (a = t.alternate, t.child !== null || a !== null && a.child !== null)
          for (l = k0(l); l !== null; ) {
            if (a = l[Gl]) return a;
            l = k0(l);
          }
        return t;
      }
      l = a, a = l.parentNode;
    }
    return null;
  }
  function wa(l) {
    if (l = l[Gl] || l[Ka]) {
      var t = l.tag;
      if (t === 5 || t === 6 || t === 13 || t === 31 || t === 26 || t === 27 || t === 3)
        return l;
    }
    return null;
  }
  function xu(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l.stateNode;
    throw Error(m(33));
  }
  function ka(l) {
    var t = l[Of];
    return t || (t = l[Of] = { hoistableStyles: /* @__PURE__ */ new Map(), hoistableScripts: /* @__PURE__ */ new Map() }), t;
  }
  function Hl(l) {
    l[Ru] = !0;
  }
  var Df = /* @__PURE__ */ new Set(), Uf = {};
  function Da(l, t) {
    Wa(l, t), Wa(l + "Capture", t);
  }
  function Wa(l, t) {
    for (Uf[l] = t, l = 0; l < t.length; l++)
      Df.add(t[l]);
  }
  var Ld = RegExp(
    "^[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD][:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\-.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]*$"
  ), Nf = {}, jf = {};
  function Vd(l) {
    return wn.call(jf, l) ? !0 : wn.call(Nf, l) ? !1 : Ld.test(l) ? jf[l] = !0 : (Nf[l] = !0, !1);
  }
  function je(l, t, a) {
    if (Vd(t))
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
  function gt(l) {
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
  function Hf(l) {
    var t = l.type;
    return (l = l.nodeName) && l.toLowerCase() === "input" && (t === "checkbox" || t === "radio");
  }
  function Kd(l, t, a) {
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
      var t = Hf(l) ? "checked" : "value";
      l._valueTracker = Kd(
        l,
        t,
        "" + l[t]
      );
    }
  }
  function Rf(l) {
    if (!l) return !1;
    var t = l._valueTracker;
    if (!t) return !0;
    var a = t.getValue(), u = "";
    return l && (u = Hf(l) ? l.checked ? "true" : "false" : l.value), l = u, l !== a ? (t.setValue(l), !0) : !1;
  }
  function Re(l) {
    if (l = l || (typeof document < "u" ? document : void 0), typeof l > "u") return null;
    try {
      return l.activeElement || l.body;
    } catch {
      return l.body;
    }
  }
  var Jd = /[\n"\\]/g;
  function St(l) {
    return l.replace(
      Jd,
      function(t) {
        return "\\" + t.charCodeAt(0).toString(16) + " ";
      }
    );
  }
  function ac(l, t, a, u, e, n, c, i) {
    l.name = "", c != null && typeof c != "function" && typeof c != "symbol" && typeof c != "boolean" ? l.type = c : l.removeAttribute("type"), t != null ? c === "number" ? (t === 0 && l.value === "" || l.value != t) && (l.value = "" + gt(t)) : l.value !== "" + gt(t) && (l.value = "" + gt(t)) : c !== "submit" && c !== "reset" || l.removeAttribute("value"), t != null ? uc(l, c, gt(t)) : a != null ? uc(l, c, gt(a)) : u != null && l.removeAttribute("value"), e == null && n != null && (l.defaultChecked = !!n), e != null && (l.checked = e && typeof e != "function" && typeof e != "symbol"), i != null && typeof i != "function" && typeof i != "symbol" && typeof i != "boolean" ? l.name = "" + gt(i) : l.removeAttribute("name");
  }
  function xf(l, t, a, u, e, n, c, i) {
    if (n != null && typeof n != "function" && typeof n != "symbol" && typeof n != "boolean" && (l.type = n), t != null || a != null) {
      if (!(n !== "submit" && n !== "reset" || t != null)) {
        tc(l);
        return;
      }
      a = a != null ? "" + gt(a) : "", t = t != null ? "" + gt(t) : a, i || t === l.value || (l.value = t), l.defaultValue = t;
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
      for (a = "" + gt(a), t = null, e = 0; e < l.length; e++) {
        if (l[e].value === a) {
          l[e].selected = !0, u && (l[e].defaultSelected = !0);
          return;
        }
        t !== null || l[e].disabled || (t = l[e]);
      }
      t !== null && (t.selected = !0);
    }
  }
  function qf(l, t, a) {
    if (t != null && (t = "" + gt(t), t !== l.value && (l.value = t), a == null)) {
      l.defaultValue !== t && (l.defaultValue = t);
      return;
    }
    l.defaultValue = a != null ? "" + gt(a) : "";
  }
  function Bf(l, t, a, u) {
    if (t == null) {
      if (u != null) {
        if (a != null) throw Error(m(92));
        if (tl(u)) {
          if (1 < u.length) throw Error(m(93));
          u = u[0];
        }
        a = u;
      }
      a == null && (a = ""), t = a;
    }
    a = gt(t), l.defaultValue = a, u = l.textContent, u === a && u !== "" && u !== null && (l.value = u), tc(l);
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
  var wd = new Set(
    "animationIterationCount aspectRatio borderImageOutset borderImageSlice borderImageWidth boxFlex boxFlexGroup boxOrdinalGroup columnCount columns flex flexGrow flexPositive flexShrink flexNegative flexOrder gridArea gridRow gridRowEnd gridRowSpan gridRowStart gridColumn gridColumnEnd gridColumnSpan gridColumnStart fontWeight lineClamp lineHeight opacity order orphans scale tabSize widows zIndex zoom fillOpacity floodOpacity stopOpacity strokeDasharray strokeDashoffset strokeMiterlimit strokeOpacity strokeWidth MozAnimationIterationCount MozBoxFlex MozBoxFlexGroup MozLineClamp msAnimationIterationCount msFlex msZoom msFlexGrow msFlexNegative msFlexOrder msFlexPositive msFlexShrink msGridColumn msGridColumnSpan msGridRow msGridRowSpan WebkitAnimationIterationCount WebkitBoxFlex WebKitBoxFlexGroup WebkitBoxOrdinalGroup WebkitColumnCount WebkitColumns WebkitFlex WebkitFlexGrow WebkitFlexPositive WebkitFlexShrink WebkitLineClamp".split(
      " "
    )
  );
  function Cf(l, t, a) {
    var u = t.indexOf("--") === 0;
    a == null || typeof a == "boolean" || a === "" ? u ? l.setProperty(t, "") : t === "float" ? l.cssFloat = "" : l[t] = "" : u ? l.setProperty(t, a) : typeof a != "number" || a === 0 || wd.has(t) ? t === "float" ? l.cssFloat = a : l[t] = ("" + a).trim() : l[t] = a + "px";
  }
  function Yf(l, t, a) {
    if (t != null && typeof t != "object")
      throw Error(m(62));
    if (l = l.style, a != null) {
      for (var u in a)
        !a.hasOwnProperty(u) || t != null && t.hasOwnProperty(u) || (u.indexOf("--") === 0 ? l.setProperty(u, "") : u === "float" ? l.cssFloat = "" : l[u] = "");
      for (var e in t)
        u = t[e], t.hasOwnProperty(e) && a[e] !== u && Cf(l, e, u);
    } else
      for (var n in t)
        t.hasOwnProperty(n) && Cf(l, n, t[n]);
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
  var kd = /* @__PURE__ */ new Map([
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
  ]), Wd = /^[\u0000-\u001F ]*j[\r\n\t]*a[\r\n\t]*v[\r\n\t]*a[\r\n\t]*s[\r\n\t]*c[\r\n\t]*r[\r\n\t]*i[\r\n\t]*p[\r\n\t]*t[\r\n\t]*:/i;
  function xe(l) {
    return Wd.test("" + l) ? "javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')" : l;
  }
  function Yt() {
  }
  var nc = null;
  function cc(l) {
    return l = l.target || l.srcElement || window, l.correspondingUseElement && (l = l.correspondingUseElement), l.nodeType === 3 ? l.parentNode : l;
  }
  var Ia = null, Pa = null;
  function Gf(l) {
    var t = wa(l);
    if (t && (l = t.stateNode)) {
      var a = l[Wl] || null;
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
              'input[name="' + St(
                "" + t
              ) + '"][type="radio"]'
            ), t = 0; t < a.length; t++) {
              var u = a[t];
              if (u !== l && u.form === l.form) {
                var e = u[Wl] || null;
                if (!e) throw Error(m(90));
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
              u = a[t], u.form === l.form && Rf(u);
          }
          break l;
        case "textarea":
          qf(l, a.value, a.defaultValue);
          break l;
        case "select":
          t = a.value, t != null && $a(l, !!a.multiple, t, !1);
      }
    }
  }
  var ic = !1;
  function Xf(l, t, a) {
    if (ic) return l(t, a);
    ic = !0;
    try {
      var u = l(t);
      return u;
    } finally {
      if (ic = !1, (Ia !== null || Pa !== null) && (Tn(), Ia && (t = Ia, l = Pa, Pa = Ia = null, Gf(t), l)))
        for (t = 0; t < l.length; t++) Gf(l[t]);
    }
  }
  function qu(l, t) {
    var a = l.stateNode;
    if (a === null) return null;
    var u = a[Wl] || null;
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
        m(231, t, typeof a)
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
  function Qf() {
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
  function Zf() {
    return !1;
  }
  function $l(l) {
    function t(a, u, e, n, c) {
      this._reactName = a, this._targetInst = e, this.type = u, this.nativeEvent = n, this.target = c, this.currentTarget = null;
      for (var i in l)
        l.hasOwnProperty(i) && (a = l[i], this[i] = a ? a(n) : n[i]);
      return this.isDefaultPrevented = (n.defaultPrevented != null ? n.defaultPrevented : n.returnValue === !1) ? Ce : Zf, this.isPropagationStopped = Zf, this;
    }
    return q(t.prototype, {
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
  }, Ye = $l(Ua), Cu = q({}, Ua, { view: 0, detail: 0 }), $d = $l(Cu), vc, dc, Yu, Ge = q({}, Cu, {
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
      return "movementX" in l ? l.movementX : (l !== Yu && (Yu && l.type === "mousemove" ? (vc = l.screenX - Yu.screenX, dc = l.screenY - Yu.screenY) : dc = vc = 0, Yu = l), vc);
    },
    movementY: function(l) {
      return "movementY" in l ? l.movementY : dc;
    }
  }), Lf = $l(Ge), Fd = q({}, Ge, { dataTransfer: 0 }), Id = $l(Fd), Pd = q({}, Cu, { relatedTarget: 0 }), oc = $l(Pd), lo = q({}, Ua, {
    animationName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), to = $l(lo), ao = q({}, Ua, {
    clipboardData: function(l) {
      return "clipboardData" in l ? l.clipboardData : window.clipboardData;
    }
  }), uo = $l(ao), eo = q({}, Ua, { data: 0 }), Vf = $l(eo), no = {
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
  }, co = {
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
  }, io = {
    Alt: "altKey",
    Control: "ctrlKey",
    Meta: "metaKey",
    Shift: "shiftKey"
  };
  function fo(l) {
    var t = this.nativeEvent;
    return t.getModifierState ? t.getModifierState(l) : (l = io[l]) ? !!t[l] : !1;
  }
  function yc() {
    return fo;
  }
  var so = q({}, Cu, {
    key: function(l) {
      if (l.key) {
        var t = no[l.key] || l.key;
        if (t !== "Unidentified") return t;
      }
      return l.type === "keypress" ? (l = Be(l), l === 13 ? "Enter" : String.fromCharCode(l)) : l.type === "keydown" || l.type === "keyup" ? co[l.keyCode] || "Unidentified" : "";
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
  }), vo = $l(so), oo = q({}, Ge, {
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
  }), Kf = $l(oo), yo = q({}, Cu, {
    touches: 0,
    targetTouches: 0,
    changedTouches: 0,
    altKey: 0,
    metaKey: 0,
    ctrlKey: 0,
    shiftKey: 0,
    getModifierState: yc
  }), mo = $l(yo), ho = q({}, Ua, {
    propertyName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), go = $l(ho), So = q({}, Ge, {
    deltaX: function(l) {
      return "deltaX" in l ? l.deltaX : "wheelDeltaX" in l ? -l.wheelDeltaX : 0;
    },
    deltaY: function(l) {
      return "deltaY" in l ? l.deltaY : "wheelDeltaY" in l ? -l.wheelDeltaY : "wheelDelta" in l ? -l.wheelDelta : 0;
    },
    deltaZ: 0,
    deltaMode: 0
  }), ro = $l(So), bo = q({}, Ua, {
    newState: 0,
    oldState: 0
  }), _o = $l(bo), zo = [9, 13, 27, 32], mc = Gt && "CompositionEvent" in window, Gu = null;
  Gt && "documentMode" in document && (Gu = document.documentMode);
  var To = Gt && "TextEvent" in window && !Gu, Jf = Gt && (!mc || Gu && 8 < Gu && 11 >= Gu), wf = " ", kf = !1;
  function Wf(l, t) {
    switch (l) {
      case "keyup":
        return zo.indexOf(t.keyCode) !== -1;
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
  function $f(l) {
    return l = l.detail, typeof l == "object" && "data" in l ? l.data : null;
  }
  var lu = !1;
  function Eo(l, t) {
    switch (l) {
      case "compositionend":
        return $f(t);
      case "keypress":
        return t.which !== 32 ? null : (kf = !0, wf);
      case "textInput":
        return l = t.data, l === wf && kf ? null : l;
      default:
        return null;
    }
  }
  function Ao(l, t) {
    if (lu)
      return l === "compositionend" || !mc && Wf(l, t) ? (l = Qf(), qe = sc = ua = null, lu = !1, l) : null;
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
        return Jf && t.locale !== "ko" ? null : t.data;
      default:
        return null;
    }
  }
  var po = {
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
  function Ff(l) {
    var t = l && l.nodeName && l.nodeName.toLowerCase();
    return t === "input" ? !!po[l.type] : t === "textarea";
  }
  function If(l, t, a, u) {
    Ia ? Pa ? Pa.push(u) : Pa = [u] : Ia = u, t = Un(t, "onChange"), 0 < t.length && (a = new Ye(
      "onChange",
      "change",
      null,
      a,
      u
    ), l.push({ event: a, listeners: t }));
  }
  var Xu = null, Qu = null;
  function Mo(l) {
    x0(l, 0);
  }
  function Xe(l) {
    var t = xu(l);
    if (Rf(t)) return l;
  }
  function Pf(l, t) {
    if (l === "change") return t;
  }
  var ls = !1;
  if (Gt) {
    var hc;
    if (Gt) {
      var gc = "oninput" in document;
      if (!gc) {
        var ts = document.createElement("div");
        ts.setAttribute("oninput", "return;"), gc = typeof ts.oninput == "function";
      }
      hc = gc;
    } else hc = !1;
    ls = hc && (!document.documentMode || 9 < document.documentMode);
  }
  function as() {
    Xu && (Xu.detachEvent("onpropertychange", us), Qu = Xu = null);
  }
  function us(l) {
    if (l.propertyName === "value" && Xe(Qu)) {
      var t = [];
      If(
        t,
        Qu,
        l,
        cc(l)
      ), Xf(Mo, t);
    }
  }
  function Oo(l, t, a) {
    l === "focusin" ? (as(), Xu = t, Qu = a, Xu.attachEvent("onpropertychange", us)) : l === "focusout" && as();
  }
  function Do(l) {
    if (l === "selectionchange" || l === "keyup" || l === "keydown")
      return Xe(Qu);
  }
  function Uo(l, t) {
    if (l === "click") return Xe(t);
  }
  function No(l, t) {
    if (l === "input" || l === "change")
      return Xe(t);
  }
  function jo(l, t) {
    return l === t && (l !== 0 || 1 / l === 1 / t) || l !== l && t !== t;
  }
  var ft = typeof Object.is == "function" ? Object.is : jo;
  function Zu(l, t) {
    if (ft(l, t)) return !0;
    if (typeof l != "object" || l === null || typeof t != "object" || t === null)
      return !1;
    var a = Object.keys(l), u = Object.keys(t);
    if (a.length !== u.length) return !1;
    for (u = 0; u < a.length; u++) {
      var e = a[u];
      if (!wn.call(t, e) || !ft(l[e], t[e]))
        return !1;
    }
    return !0;
  }
  function es(l) {
    for (; l && l.firstChild; ) l = l.firstChild;
    return l;
  }
  function ns(l, t) {
    var a = es(l);
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
      a = es(a);
    }
  }
  function cs(l, t) {
    return l && t ? l === t ? !0 : l && l.nodeType === 3 ? !1 : t && t.nodeType === 3 ? cs(l, t.parentNode) : "contains" in l ? l.contains(t) : l.compareDocumentPosition ? !!(l.compareDocumentPosition(t) & 16) : !1 : !1;
  }
  function is(l) {
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
  var Ho = Gt && "documentMode" in document && 11 >= document.documentMode, tu = null, rc = null, Lu = null, bc = !1;
  function fs(l, t, a) {
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
  }, _c = {}, ss = {};
  Gt && (ss = document.createElement("div").style, "AnimationEvent" in window || (delete au.animationend.animation, delete au.animationiteration.animation, delete au.animationstart.animation), "TransitionEvent" in window || delete au.transitionend.transition);
  function ja(l) {
    if (_c[l]) return _c[l];
    if (!au[l]) return l;
    var t = au[l], a;
    for (a in t)
      if (t.hasOwnProperty(a) && a in ss)
        return _c[l] = t[a];
    return l;
  }
  var vs = ja("animationend"), ds = ja("animationiteration"), os = ja("animationstart"), Ro = ja("transitionrun"), xo = ja("transitionstart"), qo = ja("transitioncancel"), ys = ja("transitionend"), ms = /* @__PURE__ */ new Map(), zc = "abort auxClick beforeToggle cancel canPlay canPlayThrough click close contextMenu copy cut drag dragEnd dragEnter dragExit dragLeave dragOver dragStart drop durationChange emptied encrypted ended error gotPointerCapture input invalid keyDown keyPress keyUp load loadedData loadedMetadata loadStart lostPointerCapture mouseDown mouseMove mouseOut mouseOver mouseUp paste pause play playing pointerCancel pointerDown pointerMove pointerOut pointerOver pointerUp progress rateChange reset resize seeked seeking stalled submit suspend timeUpdate touchCancel touchEnd touchStart volumeChange scroll toggle touchMove waiting wheel".split(
    " "
  );
  zc.push("scrollEnd");
  function Ot(l, t) {
    ms.set(l, t), Da(t, [l]);
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
  }, rt = [], uu = 0, Tc = 0;
  function Ze() {
    for (var l = uu, t = Tc = uu = 0; t < l; ) {
      var a = rt[t];
      rt[t++] = null;
      var u = rt[t];
      rt[t++] = null;
      var e = rt[t];
      rt[t++] = null;
      var n = rt[t];
      if (rt[t++] = null, u !== null && e !== null) {
        var c = u.pending;
        c === null ? e.next = e : (e.next = c.next, c.next = e), u.pending = e;
      }
      n !== 0 && hs(a, e, n);
    }
  }
  function Le(l, t, a, u) {
    rt[uu++] = l, rt[uu++] = t, rt[uu++] = a, rt[uu++] = u, Tc |= u, l.lanes |= u, l = l.alternate, l !== null && (l.lanes |= u);
  }
  function Ec(l, t, a, u) {
    return Le(l, t, a, u), Ve(l);
  }
  function Ha(l, t) {
    return Le(l, null, null, t), Ve(l);
  }
  function hs(l, t, a) {
    l.lanes |= a;
    var u = l.alternate;
    u !== null && (u.lanes |= a);
    for (var e = !1, n = l.return; n !== null; )
      n.childLanes |= a, u = n.alternate, u !== null && (u.childLanes |= a), n.tag === 22 && (l = n.stateNode, l === null || l._visibility & 1 || (e = !0)), l = n, n = n.return;
    return l.tag === 3 ? (n = l.stateNode, e && t !== null && (e = 31 - it(a), l = n.hiddenUpdates, u = l[e], u === null ? l[e] = [t] : u.push(t), t.lane = a | 536870912), n) : null;
  }
  function Ve(l) {
    if (50 < de)
      throw de = 0, Hi = null, Error(m(185));
    for (var t = l.return; t !== null; )
      l = t, t = l.return;
    return l.tag === 3 ? l.stateNode : null;
  }
  var eu = {};
  function Bo(l, t, a, u) {
    this.tag = l, this.key = a, this.sibling = this.child = this.return = this.stateNode = this.type = this.elementType = null, this.index = 0, this.refCleanup = this.ref = null, this.pendingProps = t, this.dependencies = this.memoizedState = this.updateQueue = this.memoizedProps = null, this.mode = u, this.subtreeFlags = this.flags = 0, this.deletions = null, this.childLanes = this.lanes = 0, this.alternate = null;
  }
  function st(l, t, a, u) {
    return new Bo(l, t, a, u);
  }
  function Ac(l) {
    return l = l.prototype, !(!l || !l.isReactComponent);
  }
  function Xt(l, t) {
    var a = l.alternate;
    return a === null ? (a = st(
      l.tag,
      t,
      l.key,
      l.mode
    ), a.elementType = l.elementType, a.type = l.type, a.stateNode = l.stateNode, a.alternate = l, l.alternate = a) : (a.pendingProps = t, a.type = l.type, a.flags = 0, a.subtreeFlags = 0, a.deletions = null), a.flags = l.flags & 65011712, a.childLanes = l.childLanes, a.lanes = l.lanes, a.child = l.child, a.memoizedProps = l.memoizedProps, a.memoizedState = l.memoizedState, a.updateQueue = l.updateQueue, t = l.dependencies, a.dependencies = t === null ? null : { lanes: t.lanes, firstContext: t.firstContext }, a.sibling = l.sibling, a.index = l.index, a.ref = l.ref, a.refCleanup = l.refCleanup, a;
  }
  function gs(l, t) {
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
      c = Qy(
        l,
        a,
        j.current
      ) ? 26 : l === "html" || l === "head" || l === "body" ? 27 : 5;
    else
      l: switch (l) {
        case ut:
          return l = st(31, a, t, e), l.elementType = ut, l.lanes = n, l;
        case pl:
          return Ra(a.children, e, n, t);
        case ht:
          c = 8, e |= 24;
          break;
        case Vl:
          return l = st(12, a, t, e | 2), l.elementType = Vl, l.lanes = n, l;
        case at:
          return l = st(13, a, t, e), l.elementType = at, l.lanes = n, l;
        case Bl:
          return l = st(19, a, t, e), l.elementType = Bl, l.lanes = n, l;
        default:
          if (typeof l == "object" && l !== null)
            switch (l.$$typeof) {
              case Nl:
                c = 10;
                break l;
              case Mt:
                c = 9;
                break l;
              case Jl:
                c = 11;
                break l;
              case L:
                c = 14;
                break l;
              case Cl:
                c = 16, u = null;
                break l;
            }
          c = 29, a = Error(
            m(130, l === null ? "null" : typeof l, "")
          ), u = null;
      }
    return t = st(c, a, t, e), t.elementType = l, t.type = u, t.lanes = n, t;
  }
  function Ra(l, t, a, u) {
    return l = st(7, l, u, t), l.lanes = a, l;
  }
  function pc(l, t, a) {
    return l = st(6, l, null, t), l.lanes = a, l;
  }
  function Ss(l) {
    var t = st(18, null, null, 0);
    return t.stateNode = l, t;
  }
  function Mc(l, t, a) {
    return t = st(
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
  var rs = /* @__PURE__ */ new WeakMap();
  function bt(l, t) {
    if (typeof l == "object" && l !== null) {
      var a = rs.get(l);
      return a !== void 0 ? a : (t = {
        value: l,
        source: t,
        stack: Sf(t)
      }, rs.set(l, t), t);
    }
    return {
      value: l,
      source: t,
      stack: Sf(t)
    };
  }
  var nu = [], cu = 0, Je = null, Vu = 0, _t = [], zt = 0, ea = null, jt = 1, Ht = "";
  function Qt(l, t) {
    nu[cu++] = Vu, nu[cu++] = Je, Je = l, Vu = t;
  }
  function bs(l, t, a) {
    _t[zt++] = jt, _t[zt++] = Ht, _t[zt++] = ea, ea = l;
    var u = jt;
    l = Ht;
    var e = 32 - it(u) - 1;
    u &= ~(1 << e), a += 1;
    var n = 32 - it(t) + e;
    if (30 < n) {
      var c = e - e % 5;
      n = (u & (1 << c) - 1).toString(32), u >>= c, e -= c, jt = 1 << 32 - it(t) + e | a << e | u, Ht = n + l;
    } else
      jt = 1 << n | a << e | u, Ht = l;
  }
  function Oc(l) {
    l.return !== null && (Qt(l, 1), bs(l, 1, 0));
  }
  function Dc(l) {
    for (; l === Je; )
      Je = nu[--cu], nu[cu] = null, Vu = nu[--cu], nu[cu] = null;
    for (; l === ea; )
      ea = _t[--zt], _t[zt] = null, Ht = _t[--zt], _t[zt] = null, jt = _t[--zt], _t[zt] = null;
  }
  function _s(l, t) {
    _t[zt++] = jt, _t[zt++] = Ht, _t[zt++] = ea, jt = t.id, Ht = t.overflow, ea = l;
  }
  var Xl = null, ml = null, $ = !1, na = null, Tt = !1, Uc = Error(m(519));
  function ca(l) {
    var t = Error(
      m(
        418,
        1 < arguments.length && arguments[1] !== void 0 && arguments[1] ? "text" : "HTML",
        ""
      )
    );
    throw Ku(bt(t, l)), Uc;
  }
  function zs(l) {
    var t = l.stateNode, a = l.type, u = l.memoizedProps;
    switch (t[Gl] = l, t[Wl] = u, a) {
      case "dialog":
        J("cancel", t), J("close", t);
        break;
      case "iframe":
      case "object":
      case "embed":
        J("load", t);
        break;
      case "video":
      case "audio":
        for (a = 0; a < ye.length; a++)
          J(ye[a], t);
        break;
      case "source":
        J("error", t);
        break;
      case "img":
      case "image":
      case "link":
        J("error", t), J("load", t);
        break;
      case "details":
        J("toggle", t);
        break;
      case "input":
        J("invalid", t), xf(
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
        J("invalid", t);
        break;
      case "textarea":
        J("invalid", t), Bf(t, u.value, u.defaultValue, u.children);
    }
    a = u.children, typeof a != "string" && typeof a != "number" && typeof a != "bigint" || t.textContent === "" + a || u.suppressHydrationWarning === !0 || Y0(t.textContent, a) ? (u.popover != null && (J("beforetoggle", t), J("toggle", t)), u.onScroll != null && J("scroll", t), u.onScrollEnd != null && J("scrollend", t), u.onClick != null && (t.onclick = Yt), t = !0) : t = !1, t || ca(l, !0);
  }
  function Ts(l) {
    for (Xl = l.return; Xl; )
      switch (Xl.tag) {
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
          Xl = Xl.return;
      }
  }
  function iu(l) {
    if (l !== Xl) return !1;
    if (!$) return Ts(l), $ = !0, !1;
    var t = l.tag, a;
    if ((a = t !== 3 && t !== 27) && ((a = t === 5) && (a = l.type, a = !(a !== "form" && a !== "button") || wi(l.type, l.memoizedProps)), a = !a), a && ml && ca(l), Ts(l), t === 13) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(m(317));
      ml = w0(l);
    } else if (t === 31) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(m(317));
      ml = w0(l);
    } else
      t === 27 ? (t = ml, _a(l.type) ? (l = Ii, Ii = null, ml = l) : ml = t) : ml = Xl ? At(l.stateNode.nextSibling) : null;
    return !0;
  }
  function xa() {
    ml = Xl = null, $ = !1;
  }
  function Nc() {
    var l = na;
    return l !== null && (lt === null ? lt = l : lt.push.apply(
      lt,
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
    l._currentValue = jc.current, T(jc);
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
        if (c = e.return, c === null) throw Error(m(341));
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
        if (c === null) throw Error(m(387));
        if (c = c.memoizedProps, c !== null) {
          var i = e.type;
          ft(e.pendingProps.value, c.value) || (l !== null ? l.push(i) : l = [i]);
        }
      } else if (e === al.current) {
        if (c = e.alternate, c === null) throw Error(m(387));
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
      if (!ft(
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
  function Ql(l) {
    return Es(qa, l);
  }
  function ke(l, t) {
    return qa === null && Ba(l), Es(l, t);
  }
  function Es(l, t) {
    var a = t._currentValue;
    if (t = { context: t, memoizedValue: a, next: null }, Zt === null) {
      if (l === null) throw Error(m(308));
      Zt = t, l.dependencies = { lanes: 0, firstContext: t }, l.flags |= 524288;
    } else Zt = Zt.next = t;
    return a;
  }
  var Co = typeof AbortController < "u" ? AbortController : function() {
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
  }, Yo = _.unstable_scheduleCallback, Go = _.unstable_NormalPriority, Ml = {
    $$typeof: Nl,
    Consumer: null,
    Provider: null,
    _currentValue: null,
    _currentValue2: null,
    _threadCount: 0
  };
  function xc() {
    return {
      controller: new Co(),
      data: /* @__PURE__ */ new Map(),
      refCount: 0
    };
  }
  function Ju(l) {
    l.refCount--, l.refCount === 0 && Yo(Go, function() {
      l.controller.abort();
    });
  }
  var wu = null, qc = 0, su = 0, vu = null;
  function Xo(l, t) {
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
    return qc++, t.then(As, As), t;
  }
  function As() {
    if (--qc === 0 && wu !== null) {
      vu !== null && (vu.status = "fulfilled");
      var l = wu;
      wu = null, su = 0, vu = null;
      for (var t = 0; t < l.length; t++) (0, l[t])();
    }
  }
  function Qo(l, t) {
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
  var ps = S.S;
  S.S = function(l, t) {
    f0 = nt(), typeof t == "object" && t !== null && typeof t.then == "function" && Xo(l, t), ps !== null && ps(l, t);
  };
  var Ca = v(null);
  function Bc() {
    var l = Ca.current;
    return l !== null ? l : ol.pooledCache;
  }
  function We(l, t) {
    t === null ? O(Ca, Ca.current) : O(Ca, t.pool);
  }
  function Ms() {
    var l = Bc();
    return l === null ? null : { parent: Ml._currentValue, pool: l };
  }
  var du = Error(m(460)), Cc = Error(m(474)), $e = Error(m(542)), Fe = { then: function() {
  } };
  function Os(l) {
    return l = l.status, l === "fulfilled" || l === "rejected";
  }
  function Ds(l, t, a) {
    switch (a = l[a], a === void 0 ? l.push(t) : a !== t && (t.then(Yt, Yt), t = a), t.status) {
      case "fulfilled":
        return t.value;
      case "rejected":
        throw l = t.reason, Ns(l), l;
      default:
        if (typeof t.status == "string") t.then(Yt, Yt);
        else {
          if (l = ol, l !== null && 100 < l.shellSuspendCounter)
            throw Error(m(482));
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
            throw l = t.reason, Ns(l), l;
        }
        throw Ga = t, du;
    }
  }
  function Ya(l) {
    try {
      var t = l._init;
      return t(l._payload);
    } catch (a) {
      throw a !== null && typeof a == "object" && typeof a.then == "function" ? (Ga = a, du) : a;
    }
  }
  var Ga = null;
  function Us() {
    if (Ga === null) throw Error(m(459));
    var l = Ga;
    return Ga = null, l;
  }
  function Ns(l) {
    if (l === du || l === $e)
      throw Error(m(483));
  }
  var ou = null, ku = 0;
  function Ie(l) {
    var t = ku;
    return ku += 1, ou === null && (ou = []), Ds(ou, l, t);
  }
  function Wu(l, t) {
    t = t.props.ref, l.ref = t !== void 0 ? t : null;
  }
  function Pe(l, t) {
    throw t.$$typeof === P ? Error(m(525)) : (l = Object.prototype.toString.call(t), Error(
      m(
        31,
        l === "[object Object]" ? "object with keys {" + Object.keys(t).join(", ") + "}" : l
      )
    ));
  }
  function js(l) {
    function t(d, s) {
      if (l) {
        var o = d.deletions;
        o === null ? (d.deletions = [s], d.flags |= 16) : o.push(s);
      }
    }
    function a(d, s) {
      if (!l) return null;
      for (; s !== null; )
        t(d, s), s = s.sibling;
      return null;
    }
    function u(d) {
      for (var s = /* @__PURE__ */ new Map(); d !== null; )
        d.key !== null ? s.set(d.key, d) : s.set(d.index, d), d = d.sibling;
      return s;
    }
    function e(d, s) {
      return d = Xt(d, s), d.index = 0, d.sibling = null, d;
    }
    function n(d, s, o) {
      return d.index = o, l ? (o = d.alternate, o !== null ? (o = o.index, o < s ? (d.flags |= 67108866, s) : o) : (d.flags |= 67108866, s)) : (d.flags |= 1048576, s);
    }
    function c(d) {
      return l && d.alternate === null && (d.flags |= 67108866), d;
    }
    function i(d, s, o, b) {
      return s === null || s.tag !== 6 ? (s = pc(o, d.mode, b), s.return = d, s) : (s = e(s, o), s.return = d, s);
    }
    function f(d, s, o, b) {
      var x = o.type;
      return x === pl ? r(
        d,
        s,
        o.props.children,
        b,
        o.key
      ) : s !== null && (s.elementType === x || typeof x == "object" && x !== null && x.$$typeof === Cl && Ya(x) === s.type) ? (s = e(s, o.props), Wu(s, o), s.return = d, s) : (s = Ke(
        o.type,
        o.key,
        o.props,
        null,
        d.mode,
        b
      ), Wu(s, o), s.return = d, s);
    }
    function y(d, s, o, b) {
      return s === null || s.tag !== 4 || s.stateNode.containerInfo !== o.containerInfo || s.stateNode.implementation !== o.implementation ? (s = Mc(o, d.mode, b), s.return = d, s) : (s = e(s, o.children || []), s.return = d, s);
    }
    function r(d, s, o, b, x) {
      return s === null || s.tag !== 7 ? (s = Ra(
        o,
        d.mode,
        b,
        x
      ), s.return = d, s) : (s = e(s, o), s.return = d, s);
    }
    function z(d, s, o) {
      if (typeof s == "string" && s !== "" || typeof s == "number" || typeof s == "bigint")
        return s = pc(
          "" + s,
          d.mode,
          o
        ), s.return = d, s;
      if (typeof s == "object" && s !== null) {
        switch (s.$$typeof) {
          case rl:
            return o = Ke(
              s.type,
              s.key,
              s.props,
              null,
              d.mode,
              o
            ), Wu(o, s), o.return = d, o;
          case bl:
            return s = Mc(
              s,
              d.mode,
              o
            ), s.return = d, s;
          case Cl:
            return s = Ya(s), z(d, s, o);
        }
        if (tl(s) || Yl(s))
          return s = Ra(
            s,
            d.mode,
            o,
            null
          ), s.return = d, s;
        if (typeof s.then == "function")
          return z(d, Ie(s), o);
        if (s.$$typeof === Nl)
          return z(
            d,
            ke(d, s),
            o
          );
        Pe(d, s);
      }
      return null;
    }
    function h(d, s, o, b) {
      var x = s !== null ? s.key : null;
      if (typeof o == "string" && o !== "" || typeof o == "number" || typeof o == "bigint")
        return x !== null ? null : i(d, s, "" + o, b);
      if (typeof o == "object" && o !== null) {
        switch (o.$$typeof) {
          case rl:
            return o.key === x ? f(d, s, o, b) : null;
          case bl:
            return o.key === x ? y(d, s, o, b) : null;
          case Cl:
            return o = Ya(o), h(d, s, o, b);
        }
        if (tl(o) || Yl(o))
          return x !== null ? null : r(d, s, o, b, null);
        if (typeof o.then == "function")
          return h(
            d,
            s,
            Ie(o),
            b
          );
        if (o.$$typeof === Nl)
          return h(
            d,
            s,
            ke(d, o),
            b
          );
        Pe(d, o);
      }
      return null;
    }
    function g(d, s, o, b, x) {
      if (typeof b == "string" && b !== "" || typeof b == "number" || typeof b == "bigint")
        return d = d.get(o) || null, i(s, d, "" + b, x);
      if (typeof b == "object" && b !== null) {
        switch (b.$$typeof) {
          case rl:
            return d = d.get(
              b.key === null ? o : b.key
            ) || null, f(s, d, b, x);
          case bl:
            return d = d.get(
              b.key === null ? o : b.key
            ) || null, y(s, d, b, x);
          case Cl:
            return b = Ya(b), g(
              d,
              s,
              o,
              b,
              x
            );
        }
        if (tl(b) || Yl(b))
          return d = d.get(o) || null, r(s, d, b, x, null);
        if (typeof b.then == "function")
          return g(
            d,
            s,
            o,
            Ie(b),
            x
          );
        if (b.$$typeof === Nl)
          return g(
            d,
            s,
            o,
            ke(s, b),
            x
          );
        Pe(s, b);
      }
      return null;
    }
    function U(d, s, o, b) {
      for (var x = null, F = null, R = s, Z = s = 0, W = null; R !== null && Z < o.length; Z++) {
        R.index > Z ? (W = R, R = null) : W = R.sibling;
        var I = h(
          d,
          R,
          o[Z],
          b
        );
        if (I === null) {
          R === null && (R = W);
          break;
        }
        l && R && I.alternate === null && t(d, R), s = n(I, s, Z), F === null ? x = I : F.sibling = I, F = I, R = W;
      }
      if (Z === o.length)
        return a(d, R), $ && Qt(d, Z), x;
      if (R === null) {
        for (; Z < o.length; Z++)
          R = z(d, o[Z], b), R !== null && (s = n(
            R,
            s,
            Z
          ), F === null ? x = R : F.sibling = R, F = R);
        return $ && Qt(d, Z), x;
      }
      for (R = u(R); Z < o.length; Z++)
        W = g(
          R,
          d,
          Z,
          o[Z],
          b
        ), W !== null && (l && W.alternate !== null && R.delete(
          W.key === null ? Z : W.key
        ), s = n(
          W,
          s,
          Z
        ), F === null ? x = W : F.sibling = W, F = W);
      return l && R.forEach(function(pa) {
        return t(d, pa);
      }), $ && Qt(d, Z), x;
    }
    function B(d, s, o, b) {
      if (o == null) throw Error(m(151));
      for (var x = null, F = null, R = s, Z = s = 0, W = null, I = o.next(); R !== null && !I.done; Z++, I = o.next()) {
        R.index > Z ? (W = R, R = null) : W = R.sibling;
        var pa = h(d, R, I.value, b);
        if (pa === null) {
          R === null && (R = W);
          break;
        }
        l && R && pa.alternate === null && t(d, R), s = n(pa, s, Z), F === null ? x = pa : F.sibling = pa, F = pa, R = W;
      }
      if (I.done)
        return a(d, R), $ && Qt(d, Z), x;
      if (R === null) {
        for (; !I.done; Z++, I = o.next())
          I = z(d, I.value, b), I !== null && (s = n(I, s, Z), F === null ? x = I : F.sibling = I, F = I);
        return $ && Qt(d, Z), x;
      }
      for (R = u(R); !I.done; Z++, I = o.next())
        I = g(R, d, Z, I.value, b), I !== null && (l && I.alternate !== null && R.delete(I.key === null ? Z : I.key), s = n(I, s, Z), F === null ? x = I : F.sibling = I, F = I);
      return l && R.forEach(function(Iy) {
        return t(d, Iy);
      }), $ && Qt(d, Z), x;
    }
    function vl(d, s, o, b) {
      if (typeof o == "object" && o !== null && o.type === pl && o.key === null && (o = o.props.children), typeof o == "object" && o !== null) {
        switch (o.$$typeof) {
          case rl:
            l: {
              for (var x = o.key; s !== null; ) {
                if (s.key === x) {
                  if (x = o.type, x === pl) {
                    if (s.tag === 7) {
                      a(
                        d,
                        s.sibling
                      ), b = e(
                        s,
                        o.props.children
                      ), b.return = d, d = b;
                      break l;
                    }
                  } else if (s.elementType === x || typeof x == "object" && x !== null && x.$$typeof === Cl && Ya(x) === s.type) {
                    a(
                      d,
                      s.sibling
                    ), b = e(s, o.props), Wu(b, o), b.return = d, d = b;
                    break l;
                  }
                  a(d, s);
                  break;
                } else t(d, s);
                s = s.sibling;
              }
              o.type === pl ? (b = Ra(
                o.props.children,
                d.mode,
                b,
                o.key
              ), b.return = d, d = b) : (b = Ke(
                o.type,
                o.key,
                o.props,
                null,
                d.mode,
                b
              ), Wu(b, o), b.return = d, d = b);
            }
            return c(d);
          case bl:
            l: {
              for (x = o.key; s !== null; ) {
                if (s.key === x)
                  if (s.tag === 4 && s.stateNode.containerInfo === o.containerInfo && s.stateNode.implementation === o.implementation) {
                    a(
                      d,
                      s.sibling
                    ), b = e(s, o.children || []), b.return = d, d = b;
                    break l;
                  } else {
                    a(d, s);
                    break;
                  }
                else t(d, s);
                s = s.sibling;
              }
              b = Mc(o, d.mode, b), b.return = d, d = b;
            }
            return c(d);
          case Cl:
            return o = Ya(o), vl(
              d,
              s,
              o,
              b
            );
        }
        if (tl(o))
          return U(
            d,
            s,
            o,
            b
          );
        if (Yl(o)) {
          if (x = Yl(o), typeof x != "function") throw Error(m(150));
          return o = x.call(o), B(
            d,
            s,
            o,
            b
          );
        }
        if (typeof o.then == "function")
          return vl(
            d,
            s,
            Ie(o),
            b
          );
        if (o.$$typeof === Nl)
          return vl(
            d,
            s,
            ke(d, o),
            b
          );
        Pe(d, o);
      }
      return typeof o == "string" && o !== "" || typeof o == "number" || typeof o == "bigint" ? (o = "" + o, s !== null && s.tag === 6 ? (a(d, s.sibling), b = e(s, o), b.return = d, d = b) : (a(d, s), b = pc(o, d.mode, b), b.return = d, d = b), c(d)) : a(d, s);
    }
    return function(d, s, o, b) {
      try {
        ku = 0;
        var x = vl(
          d,
          s,
          o,
          b
        );
        return ou = null, x;
      } catch (R) {
        if (R === du || R === $e) throw R;
        var F = st(29, R, null, d.mode);
        return F.lanes = b, F.return = d, F;
      } finally {
      }
    };
  }
  var Xa = js(!0), Hs = js(!1), fa = !1;
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
    if (u = u.shared, (ll & 2) !== 0) {
      var e = u.pending;
      return e === null ? t.next = t : (t.next = e.next, e.next = t), u.pending = t, t = Ve(l), hs(l, null, a), t;
    }
    return Le(l, u, t, a), Ve(l);
  }
  function $u(l, t, a) {
    if (t = t.updateQueue, t !== null && (t = t.shared, (a & 4194048) !== 0)) {
      var u = t.lanes;
      u &= l.pendingLanes, a |= u, t.lanes = a, Ef(l, a);
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
      var f = i, y = f.next;
      f.next = null, c === null ? n = y : c.next = y, c = f;
      var r = l.alternate;
      r !== null && (r = r.updateQueue, i = r.lastBaseUpdate, i !== c && (i === null ? r.firstBaseUpdate = y : i.next = y, r.lastBaseUpdate = f));
    }
    if (n !== null) {
      var z = e.baseState;
      c = 0, r = y = f = null, i = n;
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
            var U = l, B = i;
            h = t;
            var vl = a;
            switch (B.tag) {
              case 1:
                if (U = B.payload, typeof U == "function") {
                  z = U.call(vl, z, h);
                  break l;
                }
                z = U;
                break l;
              case 3:
                U.flags = U.flags & -65537 | 128;
              case 0:
                if (U = B.payload, h = typeof U == "function" ? U.call(vl, z, h) : U, h == null) break l;
                z = q({}, z, h);
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
          }, r === null ? (y = r = g, f = z) : r = r.next = g, c |= h;
        if (i = i.next, i === null) {
          if (i = e.shared.pending, i === null)
            break;
          g = i, i = g.next, g.next = null, e.lastBaseUpdate = g, e.shared.pending = null;
        }
      } while (!0);
      r === null && (f = z), e.baseState = f, e.firstBaseUpdate = y, e.lastBaseUpdate = r, n === null && (e.shared.lanes = 0), ha |= c, l.lanes = c, l.memoizedState = z;
    }
  }
  function Rs(l, t) {
    if (typeof l != "function")
      throw Error(m(191, l));
    l.call(t);
  }
  function xs(l, t) {
    var a = l.callbacks;
    if (a !== null)
      for (l.callbacks = null, l = 0; l < a.length; l++)
        Rs(a[l], t);
  }
  var yu = v(null), ln = v(0);
  function qs(l, t) {
    l = It, O(ln, l), O(yu, t), It = l | t.baseLanes;
  }
  function Zc() {
    O(ln, It), O(yu, yu.current);
  }
  function Lc() {
    It = ln.current, T(yu), T(ln);
  }
  var vt = v(null), Et = null;
  function da(l) {
    var t = l.alternate;
    O(El, El.current & 1), O(vt, l), Et === null && (t === null || yu.current !== null || t.memoizedState !== null) && (Et = l);
  }
  function Vc(l) {
    O(El, El.current), O(vt, l), Et === null && (Et = l);
  }
  function Bs(l) {
    l.tag === 22 ? (O(El, El.current), O(vt, l), Et === null && (Et = l)) : oa();
  }
  function oa() {
    O(El, El.current), O(vt, vt.current);
  }
  function dt(l) {
    T(vt), Et === l && (Et = null), T(El);
  }
  var El = v(0);
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
  var Vt = 0, X = null, fl = null, Ol = null, an = !1, mu = !1, Qa = !1, un = 0, Pu = 0, hu = null, Zo = 0;
  function _l() {
    throw Error(m(321));
  }
  function Kc(l, t) {
    if (t === null) return !1;
    for (var a = 0; a < t.length && a < l.length; a++)
      if (!ft(l[a], t[a])) return !1;
    return !0;
  }
  function Jc(l, t, a, u, e, n) {
    return Vt = n, X = t, t.memoizedState = null, t.updateQueue = null, t.lanes = 0, S.H = l === null || l.memoizedState === null ? bv : ii, Qa = !1, n = a(u, e), Qa = !1, mu && (n = Ys(
      t,
      a,
      u,
      e
    )), Cs(l), n;
  }
  function Cs(l) {
    S.H = ae;
    var t = fl !== null && fl.next !== null;
    if (Vt = 0, Ol = fl = X = null, an = !1, Pu = 0, hu = null, t) throw Error(m(300));
    l === null || Dl || (l = l.dependencies, l !== null && we(l) && (Dl = !0));
  }
  function Ys(l, t, a, u) {
    X = l;
    var e = 0;
    do {
      if (mu && (hu = null), Pu = 0, mu = !1, 25 <= e) throw Error(m(301));
      if (e += 1, Ol = fl = null, l.updateQueue != null) {
        var n = l.updateQueue;
        n.lastEffect = null, n.events = null, n.stores = null, n.memoCache != null && (n.memoCache.index = 0);
      }
      S.H = _v, n = t(a, u);
    } while (mu);
    return n;
  }
  function Lo() {
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
    Vt = 0, Ol = fl = X = null, mu = !1, Pu = un = 0, hu = null;
  }
  function kl() {
    var l = {
      memoizedState: null,
      baseState: null,
      baseQueue: null,
      queue: null,
      next: null
    };
    return Ol === null ? X.memoizedState = Ol = l : Ol = Ol.next = l, Ol;
  }
  function Al() {
    if (fl === null) {
      var l = X.alternate;
      l = l !== null ? l.memoizedState : null;
    } else l = fl.next;
    var t = Ol === null ? X.memoizedState : Ol.next;
    if (t !== null)
      Ol = t, fl = l;
    else {
      if (l === null)
        throw X.alternate === null ? Error(m(467)) : Error(m(310));
      fl = l, l = {
        memoizedState: fl.memoizedState,
        baseState: fl.baseState,
        baseQueue: fl.baseQueue,
        queue: fl.queue,
        next: null
      }, Ol === null ? X.memoizedState = Ol = l : Ol = Ol.next = l;
    }
    return Ol;
  }
  function en() {
    return { lastEffect: null, events: null, stores: null, memoCache: null };
  }
  function le(l) {
    var t = Pu;
    return Pu += 1, hu === null && (hu = []), l = Ds(hu, l, t), t = X, (Ol === null ? t.memoizedState : Ol.next) === null && (t = t.alternate, S.H = t === null || t.memoizedState === null ? bv : ii), l;
  }
  function nn(l) {
    if (l !== null && typeof l == "object") {
      if (typeof l.then == "function") return le(l);
      if (l.$$typeof === Nl) return Ql(l);
    }
    throw Error(m(438, String(l)));
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
    var t = Al();
    return Fc(t, fl, l);
  }
  function Fc(l, t, a) {
    var u = l.queue;
    if (u === null) throw Error(m(311));
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
      var i = c = null, f = null, y = t, r = !1;
      do {
        var z = y.lane & -536870913;
        if (z !== y.lane ? (k & z) === z : (Vt & z) === z) {
          var h = y.revertLane;
          if (h === 0)
            f !== null && (f = f.next = {
              lane: 0,
              revertLane: 0,
              gesture: null,
              action: y.action,
              hasEagerState: y.hasEagerState,
              eagerState: y.eagerState,
              next: null
            }), z === su && (r = !0);
          else if ((Vt & h) === h) {
            y = y.next, h === su && (r = !0);
            continue;
          } else
            z = {
              lane: 0,
              revertLane: y.revertLane,
              gesture: null,
              action: y.action,
              hasEagerState: y.hasEagerState,
              eagerState: y.eagerState,
              next: null
            }, f === null ? (i = f = z, c = n) : f = f.next = z, X.lanes |= h, ha |= h;
          z = y.action, Qa && a(n, z), n = y.hasEagerState ? y.eagerState : a(n, z);
        } else
          h = {
            lane: z,
            revertLane: y.revertLane,
            gesture: y.gesture,
            action: y.action,
            hasEagerState: y.hasEagerState,
            eagerState: y.eagerState,
            next: null
          }, f === null ? (i = f = h, c = n) : f = f.next = h, X.lanes |= z, ha |= z;
        y = y.next;
      } while (y !== null && y !== t);
      if (f === null ? c = n : f.next = i, !ft(n, l.memoizedState) && (Dl = !0, r && (a = vu, a !== null)))
        throw a;
      l.memoizedState = n, l.baseState = c, l.baseQueue = f, u.lastRenderedState = n;
    }
    return e === null && (u.lanes = 0), [l.memoizedState, u.dispatch];
  }
  function Ic(l) {
    var t = Al(), a = t.queue;
    if (a === null) throw Error(m(311));
    a.lastRenderedReducer = l;
    var u = a.dispatch, e = a.pending, n = t.memoizedState;
    if (e !== null) {
      a.pending = null;
      var c = e = e.next;
      do
        n = l(n, c.action), c = c.next;
      while (c !== e);
      ft(n, t.memoizedState) || (Dl = !0), t.memoizedState = n, t.baseQueue === null && (t.baseState = n), a.lastRenderedState = n;
    }
    return [n, u];
  }
  function Gs(l, t, a) {
    var u = X, e = Al(), n = $;
    if (n) {
      if (a === void 0) throw Error(m(407));
      a = a();
    } else a = t();
    var c = !ft(
      (fl || e).memoizedState,
      a
    );
    if (c && (e.memoizedState = a, Dl = !0), e = e.queue, ti(Zs.bind(null, u, e, l), [
      l
    ]), e.getSnapshot !== t || c || Ol !== null && Ol.memoizedState.tag & 1) {
      if (u.flags |= 2048, gu(
        9,
        { destroy: void 0 },
        Qs.bind(
          null,
          u,
          e,
          a,
          t
        ),
        null
      ), ol === null) throw Error(m(349));
      n || (Vt & 127) !== 0 || Xs(u, t, a);
    }
    return a;
  }
  function Xs(l, t, a) {
    l.flags |= 16384, l = { getSnapshot: t, value: a }, t = X.updateQueue, t === null ? (t = en(), X.updateQueue = t, t.stores = [l]) : (a = t.stores, a === null ? t.stores = [l] : a.push(l));
  }
  function Qs(l, t, a, u) {
    t.value = a, t.getSnapshot = u, Ls(t) && Vs(l);
  }
  function Zs(l, t, a) {
    return a(function() {
      Ls(t) && Vs(l);
    });
  }
  function Ls(l) {
    var t = l.getSnapshot;
    l = l.value;
    try {
      var a = t();
      return !ft(l, a);
    } catch {
      return !0;
    }
  }
  function Vs(l) {
    var t = Ha(l, 2);
    t !== null && tt(t, l, 2);
  }
  function Pc(l) {
    var t = kl();
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
  function Ks(l, t, a, u) {
    return l.baseState = a, Fc(
      l,
      fl,
      typeof u == "function" ? u : Kt
    );
  }
  function Vo(l, t, a, u, e) {
    if (vn(l)) throw Error(m(485));
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
      S.T !== null ? a(!0) : n.isTransition = !1, u(n), a = t.pending, a === null ? (n.next = t.pending = n, Js(t, n)) : (n.next = a.next, t.pending = a.next = n);
    }
  }
  function Js(l, t) {
    var a = t.action, u = t.payload, e = l.state;
    if (t.isTransition) {
      var n = S.T, c = {};
      S.T = c;
      try {
        var i = a(e, u), f = S.S;
        f !== null && f(c, i), ws(l, t, i);
      } catch (y) {
        li(l, t, y);
      } finally {
        n !== null && c.types !== null && (n.types = c.types), S.T = n;
      }
    } else
      try {
        n = a(e, u), ws(l, t, n);
      } catch (y) {
        li(l, t, y);
      }
  }
  function ws(l, t, a) {
    a !== null && typeof a == "object" && typeof a.then == "function" ? a.then(
      function(u) {
        ks(l, t, u);
      },
      function(u) {
        return li(l, t, u);
      }
    ) : ks(l, t, a);
  }
  function ks(l, t, a) {
    t.status = "fulfilled", t.value = a, Ws(t), l.state = a, t = l.pending, t !== null && (a = t.next, a === t ? l.pending = null : (a = a.next, t.next = a, Js(l, a)));
  }
  function li(l, t, a) {
    var u = l.pending;
    if (l.pending = null, u !== null) {
      u = u.next;
      do
        t.status = "rejected", t.reason = a, Ws(t), t = t.next;
      while (t !== u);
    }
    l.action = null;
  }
  function Ws(l) {
    l = l.listeners;
    for (var t = 0; t < l.length; t++) (0, l[t])();
  }
  function $s(l, t) {
    return t;
  }
  function Fs(l, t) {
    if ($) {
      var a = ol.formState;
      if (a !== null) {
        l: {
          var u = X;
          if ($) {
            if (ml) {
              t: {
                for (var e = ml, n = Tt; e.nodeType !== 8; ) {
                  if (!n) {
                    e = null;
                    break t;
                  }
                  if (e = At(
                    e.nextSibling
                  ), e === null) {
                    e = null;
                    break t;
                  }
                }
                n = e.data, e = n === "F!" || n === "F" ? e : null;
              }
              if (e) {
                ml = At(
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
    return a = kl(), a.memoizedState = a.baseState = t, u = {
      pending: null,
      lanes: 0,
      dispatch: null,
      lastRenderedReducer: $s,
      lastRenderedState: t
    }, a.queue = u, a = gv.bind(
      null,
      X,
      u
    ), u.dispatch = a, u = Pc(!1), n = ci.bind(
      null,
      X,
      !1,
      u.queue
    ), u = kl(), e = {
      state: t,
      dispatch: null,
      action: l,
      pending: null
    }, u.queue = e, a = Vo.bind(
      null,
      X,
      e,
      n,
      a
    ), e.dispatch = a, u.memoizedState = l, [t, a, !1];
  }
  function Is(l) {
    var t = Al();
    return Ps(t, fl, l);
  }
  function Ps(l, t, a) {
    if (t = Fc(
      l,
      t,
      $s
    )[0], l = cn(Kt)[0], typeof t == "object" && t !== null && typeof t.then == "function")
      try {
        var u = le(t);
      } catch (c) {
        throw c === du ? $e : c;
      }
    else u = t;
    t = Al();
    var e = t.queue, n = e.dispatch;
    return a !== t.memoizedState && (X.flags |= 2048, gu(
      9,
      { destroy: void 0 },
      Ko.bind(null, e, a),
      null
    )), [u, n, l];
  }
  function Ko(l, t) {
    l.action = t;
  }
  function lv(l) {
    var t = Al(), a = fl;
    if (a !== null)
      return Ps(t, a, l);
    Al(), t = t.memoizedState, a = Al();
    var u = a.queue.dispatch;
    return a.memoizedState = l, [t, u, !1];
  }
  function gu(l, t, a, u) {
    return l = { tag: l, create: a, deps: u, inst: t, next: null }, t = X.updateQueue, t === null && (t = en(), X.updateQueue = t), a = t.lastEffect, a === null ? t.lastEffect = l.next = l : (u = a.next, a.next = l, l.next = u, t.lastEffect = l), l;
  }
  function tv() {
    return Al().memoizedState;
  }
  function fn(l, t, a, u) {
    var e = kl();
    X.flags |= l, e.memoizedState = gu(
      1 | t,
      { destroy: void 0 },
      a,
      u === void 0 ? null : u
    );
  }
  function sn(l, t, a, u) {
    var e = Al();
    u = u === void 0 ? null : u;
    var n = e.memoizedState.inst;
    fl !== null && u !== null && Kc(u, fl.memoizedState.deps) ? e.memoizedState = gu(t, n, a, u) : (X.flags |= l, e.memoizedState = gu(
      1 | t,
      n,
      a,
      u
    ));
  }
  function av(l, t) {
    fn(8390656, 8, l, t);
  }
  function ti(l, t) {
    sn(2048, 8, l, t);
  }
  function Jo(l) {
    X.flags |= 4;
    var t = X.updateQueue;
    if (t === null)
      t = en(), X.updateQueue = t, t.events = [l];
    else {
      var a = t.events;
      a === null ? t.events = [l] : a.push(l);
    }
  }
  function uv(l) {
    var t = Al().memoizedState;
    return Jo({ ref: t, nextImpl: l }), function() {
      if ((ll & 2) !== 0) throw Error(m(440));
      return t.impl.apply(void 0, arguments);
    };
  }
  function ev(l, t) {
    return sn(4, 2, l, t);
  }
  function nv(l, t) {
    return sn(4, 4, l, t);
  }
  function cv(l, t) {
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
  function iv(l, t, a) {
    a = a != null ? a.concat([l]) : null, sn(4, 4, cv.bind(null, t, l), a);
  }
  function ai() {
  }
  function fv(l, t) {
    var a = Al();
    t = t === void 0 ? null : t;
    var u = a.memoizedState;
    return t !== null && Kc(t, u[1]) ? u[0] : (a.memoizedState = [l, t], l);
  }
  function sv(l, t) {
    var a = Al();
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
    return a === void 0 || (Vt & 1073741824) !== 0 && (k & 261930) === 0 ? l.memoizedState = t : (l.memoizedState = a, l = v0(), X.lanes |= l, ha |= l, a);
  }
  function vv(l, t, a, u) {
    return ft(a, t) ? a : yu.current !== null ? (l = ui(l, a, u), ft(l, t) || (Dl = !0), l) : (Vt & 42) === 0 || (Vt & 1073741824) !== 0 && (k & 261930) === 0 ? (Dl = !0, l.memoizedState = a) : (l = v0(), X.lanes |= l, ha |= l, t);
  }
  function dv(l, t, a, u, e) {
    var n = M.p;
    M.p = n !== 0 && 8 > n ? n : 8;
    var c = S.T, i = {};
    S.T = i, ci(l, !1, t, a);
    try {
      var f = e(), y = S.S;
      if (y !== null && y(i, f), f !== null && typeof f == "object" && typeof f.then == "function") {
        var r = Qo(
          f,
          u
        );
        te(
          l,
          t,
          r,
          mt(l)
        );
      } else
        te(
          l,
          t,
          u,
          mt(l)
        );
    } catch (z) {
      te(
        l,
        t,
        { then: function() {
        }, status: "rejected", reason: z },
        mt()
      );
    } finally {
      M.p = n, c !== null && i.types !== null && (c.types = i.types), S.T = c;
    }
  }
  function wo() {
  }
  function ei(l, t, a, u) {
    if (l.tag !== 5) throw Error(m(476));
    var e = ov(l).queue;
    dv(
      l,
      e,
      t,
      C,
      a === null ? wo : function() {
        return yv(l), a(u);
      }
    );
  }
  function ov(l) {
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
  function yv(l) {
    var t = ov(l);
    t.next === null && (t = l.alternate.memoizedState), te(
      l,
      t.next.queue,
      {},
      mt()
    );
  }
  function ni() {
    return Ql(re);
  }
  function mv() {
    return Al().memoizedState;
  }
  function hv() {
    return Al().memoizedState;
  }
  function ko(l) {
    for (var t = l.return; t !== null; ) {
      switch (t.tag) {
        case 24:
        case 3:
          var a = mt();
          l = sa(a);
          var u = va(t, l, a);
          u !== null && (tt(u, t, a), $u(u, t, a)), t = { cache: xc() }, l.payload = t;
          return;
      }
      t = t.return;
    }
  }
  function Wo(l, t, a) {
    var u = mt();
    a = {
      lane: u,
      revertLane: 0,
      gesture: null,
      action: a,
      hasEagerState: !1,
      eagerState: null,
      next: null
    }, vn(l) ? Sv(t, a) : (a = Ec(l, t, a, u), a !== null && (tt(a, l, u), rv(a, t, u)));
  }
  function gv(l, t, a) {
    var u = mt();
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
    if (vn(l)) Sv(t, e);
    else {
      var n = l.alternate;
      if (l.lanes === 0 && (n === null || n.lanes === 0) && (n = t.lastRenderedReducer, n !== null))
        try {
          var c = t.lastRenderedState, i = n(c, a);
          if (e.hasEagerState = !0, e.eagerState = i, ft(i, c))
            return Le(l, t, e, 0), ol === null && Ze(), !1;
        } catch {
        } finally {
        }
      if (a = Ec(l, t, e, u), a !== null)
        return tt(a, l, u), rv(a, t, u), !0;
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
      if (t) throw Error(m(479));
    } else
      t = Ec(
        l,
        a,
        u,
        2
      ), t !== null && tt(t, l, 2);
  }
  function vn(l) {
    var t = l.alternate;
    return l === X || t !== null && t === X;
  }
  function Sv(l, t) {
    mu = an = !0;
    var a = l.pending;
    a === null ? t.next = t : (t.next = a.next, a.next = t), l.pending = t;
  }
  function rv(l, t, a) {
    if ((a & 4194048) !== 0) {
      var u = t.lanes;
      u &= l.pendingLanes, a |= u, t.lanes = a, Ef(l, a);
    }
  }
  var ae = {
    readContext: Ql,
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
  var bv = {
    readContext: Ql,
    use: nn,
    useCallback: function(l, t) {
      return kl().memoizedState = [
        l,
        t === void 0 ? null : t
      ], l;
    },
    useContext: Ql,
    useEffect: av,
    useImperativeHandle: function(l, t, a) {
      a = a != null ? a.concat([l]) : null, fn(
        4194308,
        4,
        cv.bind(null, t, l),
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
      var a = kl();
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
      var u = kl();
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
      }, u.queue = l, l = l.dispatch = Wo.bind(
        null,
        X,
        l
      ), [u.memoizedState, l];
    },
    useRef: function(l) {
      var t = kl();
      return l = { current: l }, t.memoizedState = l;
    },
    useState: function(l) {
      l = Pc(l);
      var t = l.queue, a = gv.bind(null, X, t);
      return t.dispatch = a, [l.memoizedState, a];
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = kl();
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
      ), kl().memoizedState = l, [!1, l];
    },
    useSyncExternalStore: function(l, t, a) {
      var u = X, e = kl();
      if ($) {
        if (a === void 0)
          throw Error(m(407));
        a = a();
      } else {
        if (a = t(), ol === null)
          throw Error(m(349));
        (k & 127) !== 0 || Xs(u, t, a);
      }
      e.memoizedState = a;
      var n = { value: a, getSnapshot: t };
      return e.queue = n, av(Zs.bind(null, u, n, l), [
        l
      ]), u.flags |= 2048, gu(
        9,
        { destroy: void 0 },
        Qs.bind(
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
      var l = kl(), t = ol.identifierPrefix;
      if ($) {
        var a = Ht, u = jt;
        a = (u & ~(1 << 32 - it(u) - 1)).toString(32) + a, t = "_" + t + "R_" + a, a = un++, 0 < a && (t += "H" + a.toString(32)), t += "_";
      } else
        a = Zo++, t = "_" + t + "r_" + a.toString(32) + "_";
      return l.memoizedState = t;
    },
    useHostTransitionStatus: ni,
    useFormState: Fs,
    useActionState: Fs,
    useOptimistic: function(l) {
      var t = kl();
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
      return kl().memoizedState = ko.bind(
        null,
        X
      );
    },
    useEffectEvent: function(l) {
      var t = kl(), a = { impl: l };
      return t.memoizedState = a, function() {
        if ((ll & 2) !== 0)
          throw Error(m(440));
        return a.impl.apply(void 0, arguments);
      };
    }
  }, ii = {
    readContext: Ql,
    use: nn,
    useCallback: fv,
    useContext: Ql,
    useEffect: ti,
    useImperativeHandle: iv,
    useInsertionEffect: ev,
    useLayoutEffect: nv,
    useMemo: sv,
    useReducer: cn,
    useRef: tv,
    useState: function() {
      return cn(Kt);
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = Al();
      return vv(
        a,
        fl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = cn(Kt)[0], t = Al().memoizedState;
      return [
        typeof l == "boolean" ? l : le(l),
        t
      ];
    },
    useSyncExternalStore: Gs,
    useId: mv,
    useHostTransitionStatus: ni,
    useFormState: Is,
    useActionState: Is,
    useOptimistic: function(l, t) {
      var a = Al();
      return Ks(a, fl, l, t);
    },
    useMemoCache: $c,
    useCacheRefresh: hv
  };
  ii.useEffectEvent = uv;
  var _v = {
    readContext: Ql,
    use: nn,
    useCallback: fv,
    useContext: Ql,
    useEffect: ti,
    useImperativeHandle: iv,
    useInsertionEffect: ev,
    useLayoutEffect: nv,
    useMemo: sv,
    useReducer: Ic,
    useRef: tv,
    useState: function() {
      return Ic(Kt);
    },
    useDebugValue: ai,
    useDeferredValue: function(l, t) {
      var a = Al();
      return fl === null ? ui(a, l, t) : vv(
        a,
        fl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = Ic(Kt)[0], t = Al().memoizedState;
      return [
        typeof l == "boolean" ? l : le(l),
        t
      ];
    },
    useSyncExternalStore: Gs,
    useId: mv,
    useHostTransitionStatus: ni,
    useFormState: lv,
    useActionState: lv,
    useOptimistic: function(l, t) {
      var a = Al();
      return fl !== null ? Ks(a, fl, l, t) : (a.baseState = l, [l, a.queue.dispatch]);
    },
    useMemoCache: $c,
    useCacheRefresh: hv
  };
  _v.useEffectEvent = uv;
  function fi(l, t, a, u) {
    t = l.memoizedState, a = a(u, t), a = a == null ? t : q({}, t, a), l.memoizedState = a, l.lanes === 0 && (l.updateQueue.baseState = a);
  }
  var si = {
    enqueueSetState: function(l, t, a) {
      l = l._reactInternals;
      var u = mt(), e = sa(u);
      e.payload = t, a != null && (e.callback = a), t = va(l, e, u), t !== null && (tt(t, l, u), $u(t, l, u));
    },
    enqueueReplaceState: function(l, t, a) {
      l = l._reactInternals;
      var u = mt(), e = sa(u);
      e.tag = 1, e.payload = t, a != null && (e.callback = a), t = va(l, e, u), t !== null && (tt(t, l, u), $u(t, l, u));
    },
    enqueueForceUpdate: function(l, t) {
      l = l._reactInternals;
      var a = mt(), u = sa(a);
      u.tag = 2, t != null && (u.callback = t), t = va(l, u, a), t !== null && (tt(t, l, a), $u(t, l, a));
    }
  };
  function zv(l, t, a, u, e, n, c) {
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
      a === t && (a = q({}, a));
      for (var e in l)
        a[e] === void 0 && (a[e] = l[e]);
    }
    return a;
  }
  function Ev(l) {
    Qe(l);
  }
  function Av(l) {
    console.error(l);
  }
  function pv(l) {
    Qe(l);
  }
  function dn(l, t) {
    try {
      var a = l.onUncaughtError;
      a(t.value, { componentStack: t.stack });
    } catch (u) {
      setTimeout(function() {
        throw u;
      });
    }
  }
  function Mv(l, t, a) {
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
      dn(l, t);
    }, a;
  }
  function Ov(l) {
    return l = sa(l), l.tag = 3, l;
  }
  function Dv(l, t, a, u) {
    var e = a.type.getDerivedStateFromError;
    if (typeof e == "function") {
      var n = u.value;
      l.payload = function() {
        return e(n);
      }, l.callback = function() {
        Mv(t, a, u);
      };
    }
    var c = a.stateNode;
    c !== null && typeof c.componentDidCatch == "function" && (l.callback = function() {
      Mv(t, a, u), typeof e != "function" && (ga === null ? ga = /* @__PURE__ */ new Set([this]) : ga.add(this));
      var i = u.stack;
      this.componentDidCatch(u.value, {
        componentStack: i !== null ? i : ""
      });
    });
  }
  function $o(l, t, a, u, e) {
    if (a.flags |= 32768, u !== null && typeof u == "object" && typeof u.then == "function") {
      if (t = a.alternate, t !== null && fu(
        t,
        a,
        e,
        !0
      ), a = vt.current, a !== null) {
        switch (a.tag) {
          case 31:
          case 13:
            return Et === null ? En() : a.alternate === null && zl === 0 && (zl = 3), a.flags &= -257, a.flags |= 65536, a.lanes = e, u === Fe ? a.flags |= 16384 : (t = a.updateQueue, t === null ? a.updateQueue = /* @__PURE__ */ new Set([u]) : t.add(u), qi(l, u, e)), !1;
          case 22:
            return a.flags |= 65536, u === Fe ? a.flags |= 16384 : (t = a.updateQueue, t === null ? (t = {
              transitions: null,
              markerInstances: null,
              retryQueue: /* @__PURE__ */ new Set([u])
            }, a.updateQueue = t) : (a = t.retryQueue, a === null ? t.retryQueue = /* @__PURE__ */ new Set([u]) : a.add(u)), qi(l, u, e)), !1;
        }
        throw Error(m(435, a.tag));
      }
      return qi(l, u, e), En(), !1;
    }
    if ($)
      return t = vt.current, t !== null ? ((t.flags & 65536) === 0 && (t.flags |= 256), t.flags |= 65536, t.lanes = e, u !== Uc && (l = Error(m(422), { cause: u }), Ku(bt(l, a)))) : (u !== Uc && (t = Error(m(423), {
        cause: u
      }), Ku(
        bt(t, a)
      )), l = l.current.alternate, l.flags |= 65536, e &= -e, l.lanes |= e, u = bt(u, a), e = vi(
        l.stateNode,
        u,
        e
      ), Xc(l, e), zl !== 4 && (zl = 2)), !1;
    var n = Error(m(520), { cause: u });
    if (n = bt(n, a), ve === null ? ve = [n] : ve.push(n), zl !== 4 && (zl = 2), t === null) return !0;
    u = bt(u, a), a = t;
    do {
      switch (a.tag) {
        case 3:
          return a.flags |= 65536, l = e & -e, a.lanes |= l, l = vi(a.stateNode, u, l), Xc(a, l), !1;
        case 1:
          if (t = a.type, n = a.stateNode, (a.flags & 128) === 0 && (typeof t.getDerivedStateFromError == "function" || n !== null && typeof n.componentDidCatch == "function" && (ga === null || !ga.has(n))))
            return a.flags |= 65536, e &= -e, a.lanes |= e, e = Ov(e), Dv(
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
  var di = Error(m(461)), Dl = !1;
  function Zl(l, t, a, u) {
    t.child = l === null ? Hs(t, null, a, u) : Xa(
      t,
      l.child,
      a,
      u
    );
  }
  function Uv(l, t, a, u, e) {
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
    ), i = wc(), l !== null && !Dl ? (kc(l, t, e), Jt(l, t, e)) : ($ && i && Oc(t), t.flags |= 1, Zl(l, t, u, e), t.child);
  }
  function Nv(l, t, a, u, e) {
    if (l === null) {
      var n = a.type;
      return typeof n == "function" && !Ac(n) && n.defaultProps === void 0 && a.compare === null ? (t.tag = 15, t.type = n, jv(
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
  function jv(l, t, a, u, e) {
    if (l !== null) {
      var n = l.memoizedProps;
      if (Zu(n, u) && l.ref === t.ref)
        if (Dl = !1, t.pendingProps = u = n, bi(l, e))
          (l.flags & 131072) !== 0 && (Dl = !0);
        else
          return t.lanes = l.lanes, Jt(l, t, e);
    }
    return oi(
      l,
      t,
      a,
      u,
      e
    );
  }
  function Hv(l, t, a, u) {
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
        return Rv(
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
        ), n !== null ? qs(t, n) : Zc(), Bs(t);
      else
        return u = t.lanes = 536870912, Rv(
          l,
          t,
          n !== null ? n.baseLanes | a : a,
          a,
          u
        );
    } else
      n !== null ? (We(t, n.cachePool), qs(t, n), oa(), t.memoizedState = null) : (l !== null && We(t, null), Zc(), oa());
    return Zl(l, t, e, a), t.child;
  }
  function ue(l, t) {
    return l !== null && l.tag === 22 || t.stateNode !== null || (t.stateNode = {
      _visibility: 1,
      _pendingMarkers: null,
      _retryCache: null,
      _transitions: null
    }), t.sibling;
  }
  function Rv(l, t, a, u, e) {
    var n = Bc();
    return n = n === null ? null : { parent: Ml._currentValue, pool: n }, t.memoizedState = {
      baseLanes: a,
      cachePool: n
    }, l !== null && We(t, null), Zc(), Bs(t), l !== null && fu(l, t, u, !0), t.childLanes = e, null;
  }
  function on(l, t) {
    return t = mn(
      { mode: t.mode, children: t.children },
      l.mode
    ), t.ref = l.ref, l.child = t, t.return = l, t;
  }
  function xv(l, t, a) {
    return Xa(t, l.child, null, a), l = on(t, t.pendingProps), l.flags |= 2, dt(t), t.memoizedState = null, l;
  }
  function Fo(l, t, a) {
    var u = t.pendingProps, e = (t.flags & 128) !== 0;
    if (t.flags &= -129, l === null) {
      if ($) {
        if (u.mode === "hidden")
          return l = on(t, u), t.lanes = 536870912, ue(null, l);
        if (Vc(t), (l = ml) ? (l = J0(
          l,
          Tt
        ), l = l !== null && l.data === "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ea !== null ? { id: jt, overflow: Ht } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = Ss(l), a.return = t, t.child = a, Xl = t, ml = null)) : l = null, l === null) throw ca(t);
        return t.lanes = 536870912, null;
      }
      return on(t, u);
    }
    var n = l.memoizedState;
    if (n !== null) {
      var c = n.dehydrated;
      if (Vc(t), e)
        if (t.flags & 256)
          t.flags &= -257, t = xv(
            l,
            t,
            a
          );
        else if (t.memoizedState !== null)
          t.child = l.child, t.flags |= 128, t = null;
        else throw Error(m(558));
      else if (Dl || fu(l, t, a, !1), e = (a & l.childLanes) !== 0, Dl || e) {
        if (u = ol, u !== null && (c = Af(u, a), c !== 0 && c !== n.retryLane))
          throw n.retryLane = c, Ha(l, c), tt(u, l, c), di;
        En(), t = xv(
          l,
          t,
          a
        );
      } else
        l = n.treeContext, ml = At(c.nextSibling), Xl = t, $ = !0, na = null, Tt = !1, l !== null && _s(t, l), t = on(t, u), t.flags |= 4096;
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
        throw Error(m(284));
      (l === null || l.ref !== a) && (t.flags |= 4194816);
    }
  }
  function oi(l, t, a, u, e) {
    return Ba(t), a = Jc(
      l,
      t,
      a,
      u,
      void 0,
      e
    ), u = wc(), l !== null && !Dl ? (kc(l, t, e), Jt(l, t, e)) : ($ && u && Oc(t), t.flags |= 1, Zl(l, t, a, e), t.child);
  }
  function qv(l, t, a, u, e, n) {
    return Ba(t), t.updateQueue = null, a = Ys(
      t,
      u,
      a,
      e
    ), Cs(l), u = wc(), l !== null && !Dl ? (kc(l, t, n), Jt(l, t, n)) : ($ && u && Oc(t), t.flags |= 1, Zl(l, t, a, n), t.child);
  }
  function Bv(l, t, a, u, e) {
    if (Ba(t), t.stateNode === null) {
      var n = eu, c = a.contextType;
      typeof c == "object" && c !== null && (n = Ql(c)), n = new a(u, n), t.memoizedState = n.state !== null && n.state !== void 0 ? n.state : null, n.updater = si, t.stateNode = n, n._reactInternals = t, n = t.stateNode, n.props = u, n.state = t.memoizedState, n.refs = {}, Yc(t), c = a.contextType, n.context = typeof c == "object" && c !== null ? Ql(c) : eu, n.state = t.memoizedState, c = a.getDerivedStateFromProps, typeof c == "function" && (fi(
        t,
        a,
        c,
        u
      ), n.state = t.memoizedState), typeof a.getDerivedStateFromProps == "function" || typeof n.getSnapshotBeforeUpdate == "function" || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (c = n.state, typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount(), c !== n.state && si.enqueueReplaceState(n, n.state, null), Iu(t, u, n, e), Fu(), n.state = t.memoizedState), typeof n.componentDidMount == "function" && (t.flags |= 4194308), u = !0;
    } else if (l === null) {
      n = t.stateNode;
      var i = t.memoizedProps, f = Za(a, i);
      n.props = f;
      var y = n.context, r = a.contextType;
      c = eu, typeof r == "object" && r !== null && (c = Ql(r));
      var z = a.getDerivedStateFromProps;
      r = typeof z == "function" || typeof n.getSnapshotBeforeUpdate == "function", i = t.pendingProps !== i, r || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (i || y !== c) && Tv(
        t,
        n,
        u,
        c
      ), fa = !1;
      var h = t.memoizedState;
      n.state = h, Iu(t, u, n, e), Fu(), y = t.memoizedState, i || h !== y || fa ? (typeof z == "function" && (fi(
        t,
        a,
        z,
        u
      ), y = t.memoizedState), (f = fa || zv(
        t,
        a,
        f,
        u,
        h,
        y,
        c
      )) ? (r || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount()), typeof n.componentDidMount == "function" && (t.flags |= 4194308)) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), t.memoizedProps = u, t.memoizedState = y), n.props = u, n.state = y, n.context = c, u = f) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), u = !1);
    } else {
      n = t.stateNode, Gc(l, t), c = t.memoizedProps, r = Za(a, c), n.props = r, z = t.pendingProps, h = n.context, y = a.contextType, f = eu, typeof y == "object" && y !== null && (f = Ql(y)), i = a.getDerivedStateFromProps, (y = typeof i == "function" || typeof n.getSnapshotBeforeUpdate == "function") || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (c !== z || h !== f) && Tv(
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
      ), g = t.memoizedState), (r = fa || zv(
        t,
        a,
        r,
        u,
        h,
        g,
        f
      ) || l !== null && l.dependencies !== null && we(l.dependencies)) ? (y || typeof n.UNSAFE_componentWillUpdate != "function" && typeof n.componentWillUpdate != "function" || (typeof n.componentWillUpdate == "function" && n.componentWillUpdate(u, g, f), typeof n.UNSAFE_componentWillUpdate == "function" && n.UNSAFE_componentWillUpdate(
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
    )) : Zl(l, t, a, e), t.memoizedState = n.state, l = t.child) : l = Jt(
      l,
      t,
      e
    ), l;
  }
  function Cv(l, t, a, u) {
    return xa(), t.flags |= 256, Zl(l, t, a, u), t.child;
  }
  var yi = {
    dehydrated: null,
    treeContext: null,
    retryLane: 0,
    hydrationErrors: null
  };
  function mi(l) {
    return { baseLanes: l, cachePool: Ms() };
  }
  function hi(l, t, a) {
    return l = l !== null ? l.childLanes & ~a : 0, t && (l |= yt), l;
  }
  function Yv(l, t, a) {
    var u = t.pendingProps, e = !1, n = (t.flags & 128) !== 0, c;
    if ((c = n) || (c = l !== null && l.memoizedState === null ? !1 : (El.current & 2) !== 0), c && (e = !0, t.flags &= -129), c = (t.flags & 32) !== 0, t.flags &= -33, l === null) {
      if ($) {
        if (e ? da(t) : oa(), (l = ml) ? (l = J0(
          l,
          Tt
        ), l = l !== null && l.data !== "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ea !== null ? { id: jt, overflow: Ht } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = Ss(l), a.return = t, t.child = a, Xl = t, ml = null)) : l = null, l === null) throw ca(t);
        return Fi(l) ? t.lanes = 32 : t.lanes = 536870912, null;
      }
      var i = u.children;
      return u = u.fallback, e ? (oa(), e = t.mode, i = mn(
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
      ), t.memoizedState = yi, ue(null, u)) : (da(t), gi(t, i));
    }
    var f = l.memoizedState;
    if (f !== null && (i = f.dehydrated, i !== null)) {
      if (n)
        t.flags & 256 ? (da(t), t.flags &= -257, t = Si(
          l,
          t,
          a
        )) : t.memoizedState !== null ? (oa(), t.child = l.child, t.flags |= 128, t = null) : (oa(), i = u.fallback, e = t.mode, u = mn(
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
      else if (da(t), Fi(i)) {
        if (c = i.nextSibling && i.nextSibling.dataset, c) var y = c.dgst;
        c = y, u = Error(m(419)), u.stack = "", u.digest = c, Ku({ value: u, source: null, stack: null }), t = Si(
          l,
          t,
          a
        );
      } else if (Dl || fu(l, t, a, !1), c = (a & l.childLanes) !== 0, Dl || c) {
        if (c = ol, c !== null && (u = Af(c, a), u !== 0 && u !== f.retryLane))
          throw f.retryLane = u, Ha(l, u), tt(c, l, u), di;
        $i(i) || En(), t = Si(
          l,
          t,
          a
        );
      } else
        $i(i) ? (t.flags |= 192, t.child = l.child, t = null) : (l = f.treeContext, ml = At(
          i.nextSibling
        ), Xl = t, $ = !0, na = null, Tt = !1, l !== null && _s(t, l), t = gi(
          t,
          u.children
        ), t.flags |= 4096);
      return t;
    }
    return e ? (oa(), i = u.fallback, e = t.mode, f = l.child, y = f.sibling, u = Xt(f, {
      mode: "hidden",
      children: u.children
    }), u.subtreeFlags = f.subtreeFlags & 65011712, y !== null ? i = Xt(
      y,
      i
    ) : (i = Ra(
      i,
      e,
      a,
      null
    ), i.flags |= 2), i.return = t, u.return = t, u.sibling = i, t.child = u, ue(null, u), u = t.child, i = l.child.memoizedState, i === null ? i = mi(a) : (e = i.cachePool, e !== null ? (f = Ml._currentValue, e = e.parent !== f ? { parent: f, pool: f } : e) : e = Ms(), i = {
      baseLanes: i.baseLanes | a,
      cachePool: e
    }), u.memoizedState = i, u.childLanes = hi(
      l,
      c,
      a
    ), t.memoizedState = yi, ue(l.child, u)) : (da(t), a = l.child, l = a.sibling, a = Xt(a, {
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
    return l = st(22, l, null, t), l.lanes = 0, l;
  }
  function Si(l, t, a) {
    return Xa(t, l.child, null, a), l = gi(
      t,
      t.pendingProps.children
    ), l.flags |= 2, t.memoizedState = null, l;
  }
  function Gv(l, t, a) {
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
  function Xv(l, t, a) {
    var u = t.pendingProps, e = u.revealOrder, n = u.tail;
    u = u.children;
    var c = El.current, i = (c & 2) !== 0;
    if (i ? (c = c & 1 | 2, t.flags |= 128) : c &= 1, O(El, c), Zl(l, t, u, a), u = $ ? Vu : 0, !i && l !== null && (l.flags & 128) !== 0)
      l: for (l = t.child; l !== null; ) {
        if (l.tag === 13)
          l.memoizedState !== null && Gv(l, a, t);
        else if (l.tag === 19)
          Gv(l, a, t);
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
      throw Error(m(153));
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
  function Io(l, t, a) {
    switch (t.tag) {
      case 3:
        wl(t, t.stateNode.containerInfo), ia(t, Ml, l.memoizedState.cache), xa();
        break;
      case 27:
      case 5:
        Uu(t);
        break;
      case 4:
        wl(t, t.stateNode.containerInfo);
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
          return u.dehydrated !== null ? (da(t), t.flags |= 128, null) : (a & t.child.childLanes) !== 0 ? Yv(l, t, a) : (da(t), l = Jt(
            l,
            t,
            a
          ), l !== null ? l.sibling : null);
        da(t);
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
            return Xv(
              l,
              t,
              a
            );
          t.flags |= 128;
        }
        if (e = t.memoizedState, e !== null && (e.rendering = null, e.tail = null, e.lastEffect = null), O(El, El.current), u) break;
        return null;
      case 22:
        return t.lanes = 0, Hv(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        ia(t, Ml, l.memoizedState.cache);
    }
    return Jt(l, t, a);
  }
  function Qv(l, t, a) {
    if (l !== null)
      if (l.memoizedProps !== t.pendingProps)
        Dl = !0;
      else {
        if (!bi(l, a) && (t.flags & 128) === 0)
          return Dl = !1, Io(
            l,
            t,
            a
          );
        Dl = (l.flags & 131072) !== 0;
      }
    else
      Dl = !1, $ && (t.flags & 1048576) !== 0 && bs(t, Vu, t.index);
    switch (t.lanes = 0, t.tag) {
      case 16:
        l: {
          var u = t.pendingProps;
          if (l = Ya(t.elementType), t.type = l, typeof l == "function")
            Ac(l) ? (u = Za(l, u), t.tag = 1, t = Bv(
              null,
              t,
              l,
              u,
              a
            )) : (t.tag = 0, t = oi(
              null,
              t,
              l,
              u,
              a
            ));
          else {
            if (l != null) {
              var e = l.$$typeof;
              if (e === Jl) {
                t.tag = 11, t = Uv(
                  null,
                  t,
                  l,
                  u,
                  a
                );
                break l;
              } else if (e === L) {
                t.tag = 14, t = Nv(
                  null,
                  t,
                  l,
                  u,
                  a
                );
                break l;
              }
            }
            throw t = N(l) || l, Error(m(306, t, ""));
          }
        }
        return t;
      case 0:
        return oi(
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
        ), Bv(
          l,
          t,
          u,
          e,
          a
        );
      case 3:
        l: {
          if (wl(
            t,
            t.stateNode.containerInfo
          ), l === null) throw Error(m(387));
          u = t.pendingProps;
          var n = t.memoizedState;
          e = n.element, Gc(l, t), Iu(t, u, null, a);
          var c = t.memoizedState;
          if (u = c.cache, ia(t, Ml, u), u !== n.cache && Rc(
            t,
            [Ml],
            a,
            !0
          ), Fu(), u = c.element, n.isDehydrated)
            if (n = {
              element: u,
              isDehydrated: !1,
              cache: c.cache
            }, t.updateQueue.baseState = n, t.memoizedState = n, t.flags & 256) {
              t = Cv(
                l,
                t,
                u,
                a
              );
              break l;
            } else if (u !== e) {
              e = bt(
                Error(m(424)),
                t
              ), Ku(e), t = Cv(
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
              for (ml = At(l.firstChild), Xl = t, $ = !0, na = null, Tt = !0, a = Hs(
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
            Zl(l, t, u, a);
          }
          t = t.child;
        }
        return t;
      case 26:
        return yn(l, t), l === null ? (a = I0(
          t.type,
          null,
          t.pendingProps,
          null
        )) ? t.memoizedState = a : $ || (a = t.type, l = t.pendingProps, u = Nn(
          V.current
        ).createElement(a), u[Gl] = t, u[Wl] = l, Ll(u, a, l), Hl(u), t.stateNode = u) : t.memoizedState = I0(
          t.type,
          l.memoizedProps,
          t.pendingProps,
          l.memoizedState
        ), null;
      case 27:
        return Uu(t), l === null && $ && (u = t.stateNode = W0(
          t.type,
          t.pendingProps,
          V.current
        ), Xl = t, Tt = !0, e = ml, _a(t.type) ? (Ii = e, ml = At(u.firstChild)) : ml = e), Zl(
          l,
          t,
          t.pendingProps.children,
          a
        ), yn(l, t), l === null && (t.flags |= 4194304), t.child;
      case 5:
        return l === null && $ && ((e = u = ml) && (u = Dy(
          u,
          t.type,
          t.pendingProps,
          Tt
        ), u !== null ? (t.stateNode = u, Xl = t, ml = At(u.firstChild), Tt = !1, e = !0) : e = !1), e || ca(t)), Uu(t), e = t.type, n = t.pendingProps, c = l !== null ? l.memoizedProps : null, u = n.children, wi(e, n) ? u = null : c !== null && wi(e, c) && (t.flags |= 32), t.memoizedState !== null && (e = Jc(
          l,
          t,
          Lo,
          null,
          null,
          a
        ), re._currentValue = e), yn(l, t), Zl(l, t, u, a), t.child;
      case 6:
        return l === null && $ && ((l = a = ml) && (a = Uy(
          a,
          t.pendingProps,
          Tt
        ), a !== null ? (t.stateNode = a, Xl = t, ml = null, l = !0) : l = !1), l || ca(t)), null;
      case 13:
        return Yv(l, t, a);
      case 4:
        return wl(
          t,
          t.stateNode.containerInfo
        ), u = t.pendingProps, l === null ? t.child = Xa(
          t,
          null,
          u,
          a
        ) : Zl(l, t, u, a), t.child;
      case 11:
        return Uv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 7:
        return Zl(
          l,
          t,
          t.pendingProps,
          a
        ), t.child;
      case 8:
        return Zl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 12:
        return Zl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 10:
        return u = t.pendingProps, ia(t, t.type, u.value), Zl(l, t, u.children, a), t.child;
      case 9:
        return e = t.type._context, u = t.pendingProps.children, Ba(t), e = Ql(e), u = u(e), t.flags |= 1, Zl(l, t, u, a), t.child;
      case 14:
        return Nv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 15:
        return jv(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 19:
        return Xv(l, t, a);
      case 31:
        return Fo(l, t, a);
      case 22:
        return Hv(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        return Ba(t), u = Ql(Ml), l === null ? (e = Bc(), e === null && (e = ol, n = xc(), e.pooledCache = n, n.refCount++, n !== null && (e.pooledCacheLanes |= a), e = n), t.memoizedState = { parent: u, cache: e }, Yc(t), ia(t, Ml, e)) : ((l.lanes & a) !== 0 && (Gc(l, t), Iu(t, null, null, a), Fu()), e = l.memoizedState, n = t.memoizedState, e.parent !== u ? (e = { parent: u, cache: u }, t.memoizedState = e, t.lanes === 0 && (t.memoizedState = t.updateQueue.baseState = e), ia(t, Ml, u)) : (u = n.cache, ia(t, Ml, u), u !== e.cache && Rc(
          t,
          [Ml],
          a,
          !0
        ))), Zl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 29:
        throw t.pendingProps;
    }
    throw Error(m(156, t.tag));
  }
  function wt(l) {
    l.flags |= 4;
  }
  function _i(l, t, a, u, e) {
    if ((t = (l.mode & 32) !== 0) && (t = !1), t) {
      if (l.flags |= 16777216, (e & 335544128) === e)
        if (l.stateNode.complete) l.flags |= 8192;
        else if (m0()) l.flags |= 8192;
        else
          throw Ga = Fe, Cc;
    } else l.flags &= -16777217;
  }
  function Zv(l, t) {
    if (t.type !== "stylesheet" || (t.state.loading & 4) !== 0)
      l.flags &= -16777217;
    else if (l.flags |= 16777216, !ud(t))
      if (m0()) l.flags |= 8192;
      else
        throw Ga = Fe, Cc;
  }
  function hn(l, t) {
    t !== null && (l.flags |= 4), l.flags & 16384 && (t = l.tag !== 22 ? zf() : 536870912, l.lanes |= t, _u |= t);
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
  function hl(l) {
    var t = l.alternate !== null && l.alternate.child === l.child, a = 0, u = 0;
    if (t)
      for (var e = l.child; e !== null; )
        a |= e.lanes | e.childLanes, u |= e.subtreeFlags & 65011712, u |= e.flags & 65011712, e.return = l, e = e.sibling;
    else
      for (e = l.child; e !== null; )
        a |= e.lanes | e.childLanes, u |= e.subtreeFlags, u |= e.flags, e.return = l, e = e.sibling;
    return l.subtreeFlags |= u, l.childLanes = a, t;
  }
  function Po(l, t, a) {
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
        return hl(t), null;
      case 1:
        return hl(t), null;
      case 3:
        return a = t.stateNode, u = null, l !== null && (u = l.memoizedState.cache), t.memoizedState.cache !== u && (t.flags |= 2048), Lt(Ml), Tl(), a.pendingContext && (a.context = a.pendingContext, a.pendingContext = null), (l === null || l.child === null) && (iu(t) ? wt(t) : l === null || l.memoizedState.isDehydrated && (t.flags & 256) === 0 || (t.flags |= 1024, Nc())), hl(t), null;
      case 26:
        var e = t.type, n = t.memoizedState;
        return l === null ? (wt(t), n !== null ? (hl(t), Zv(t, n)) : (hl(t), _i(
          t,
          e,
          null,
          u,
          a
        ))) : n ? n !== l.memoizedState ? (wt(t), hl(t), Zv(t, n)) : (hl(t), t.flags &= -16777217) : (l = l.memoizedProps, l !== u && wt(t), hl(t), _i(
          t,
          e,
          l,
          u,
          a
        )), null;
      case 27:
        if (pe(t), a = V.current, e = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== u && wt(t);
        else {
          if (!u) {
            if (t.stateNode === null)
              throw Error(m(166));
            return hl(t), null;
          }
          l = j.current, iu(t) ? zs(t) : (l = W0(e, u, a), t.stateNode = l, wt(t));
        }
        return hl(t), null;
      case 5:
        if (pe(t), e = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== u && wt(t);
        else {
          if (!u) {
            if (t.stateNode === null)
              throw Error(m(166));
            return hl(t), null;
          }
          if (n = j.current, iu(t))
            zs(t);
          else {
            var c = Nn(
              V.current
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
            n[Gl] = t, n[Wl] = u;
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
            l: switch (Ll(n, e, u), e) {
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
        return hl(t), _i(
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
            throw Error(m(166));
          if (l = V.current, iu(t)) {
            if (l = t.stateNode, a = t.memoizedProps, u = null, e = Xl, e !== null)
              switch (e.tag) {
                case 27:
                case 5:
                  u = e.memoizedProps;
              }
            l[Gl] = t, l = !!(l.nodeValue === a || u !== null && u.suppressHydrationWarning === !0 || Y0(l.nodeValue, a)), l || ca(t, !0);
          } else
            l = Nn(l).createTextNode(
              u
            ), l[Gl] = t, t.stateNode = l;
        }
        return hl(t), null;
      case 31:
        if (a = t.memoizedState, l === null || l.memoizedState !== null) {
          if (u = iu(t), a !== null) {
            if (l === null) {
              if (!u) throw Error(m(318));
              if (l = t.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(m(557));
              l[Gl] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            hl(t), l = !1;
          } else
            a = Nc(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = a), l = !0;
          if (!l)
            return t.flags & 256 ? (dt(t), t) : (dt(t), null);
          if ((t.flags & 128) !== 0)
            throw Error(m(558));
        }
        return hl(t), null;
      case 13:
        if (u = t.memoizedState, l === null || l.memoizedState !== null && l.memoizedState.dehydrated !== null) {
          if (e = iu(t), u !== null && u.dehydrated !== null) {
            if (l === null) {
              if (!e) throw Error(m(318));
              if (e = t.memoizedState, e = e !== null ? e.dehydrated : null, !e) throw Error(m(317));
              e[Gl] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            hl(t), e = !1;
          } else
            e = Nc(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = e), e = !0;
          if (!e)
            return t.flags & 256 ? (dt(t), t) : (dt(t), null);
        }
        return dt(t), (t.flags & 128) !== 0 ? (t.lanes = a, t) : (a = u !== null, l = l !== null && l.memoizedState !== null, a && (u = t.child, e = null, u.alternate !== null && u.alternate.memoizedState !== null && u.alternate.memoizedState.cachePool !== null && (e = u.alternate.memoizedState.cachePool.pool), n = null, u.memoizedState !== null && u.memoizedState.cachePool !== null && (n = u.memoizedState.cachePool.pool), n !== e && (u.flags |= 2048)), a !== l && a && (t.child.flags |= 8192), hn(t, t.updateQueue), hl(t), null);
      case 4:
        return Tl(), l === null && Zi(t.stateNode.containerInfo), hl(t), null;
      case 10:
        return Lt(t.type), hl(t), null;
      case 19:
        if (T(El), u = t.memoizedState, u === null) return hl(t), null;
        if (e = (t.flags & 128) !== 0, n = u.rendering, n === null)
          if (e) ee(u, !1);
          else {
            if (zl !== 0 || l !== null && (l.flags & 128) !== 0)
              for (l = t.child; l !== null; ) {
                if (n = tn(l), n !== null) {
                  for (t.flags |= 128, ee(u, !1), l = n.updateQueue, t.updateQueue = l, hn(t, l), t.subtreeFlags = 0, l = a, a = t.child; a !== null; )
                    gs(a, l), a = a.sibling;
                  return O(
                    El,
                    El.current & 1 | 2
                  ), $ && Qt(t, u.treeForkCount), t.child;
                }
                l = l.sibling;
              }
            u.tail !== null && nt() > _n && (t.flags |= 128, e = !0, ee(u, !1), t.lanes = 4194304);
          }
        else {
          if (!e)
            if (l = tn(n), l !== null) {
              if (t.flags |= 128, e = !0, l = l.updateQueue, t.updateQueue = l, hn(t, l), ee(u, !0), u.tail === null && u.tailMode === "hidden" && !n.alternate && !$)
                return hl(t), null;
            } else
              2 * nt() - u.renderingStartTime > _n && a !== 536870912 && (t.flags |= 128, e = !0, ee(u, !1), t.lanes = 4194304);
          u.isBackwards ? (n.sibling = t.child, t.child = n) : (l = u.last, l !== null ? l.sibling = n : t.child = n, u.last = n);
        }
        return u.tail !== null ? (l = u.tail, u.rendering = l, u.tail = l.sibling, u.renderingStartTime = nt(), l.sibling = null, a = El.current, O(
          El,
          e ? a & 1 | 2 : a & 1
        ), $ && Qt(t, u.treeForkCount), l) : (hl(t), null);
      case 22:
      case 23:
        return dt(t), Lc(), u = t.memoizedState !== null, l !== null ? l.memoizedState !== null !== u && (t.flags |= 8192) : u && (t.flags |= 8192), u ? (a & 536870912) !== 0 && (t.flags & 128) === 0 && (hl(t), t.subtreeFlags & 6 && (t.flags |= 8192)) : hl(t), a = t.updateQueue, a !== null && hn(t, a.retryQueue), a = null, l !== null && l.memoizedState !== null && l.memoizedState.cachePool !== null && (a = l.memoizedState.cachePool.pool), u = null, t.memoizedState !== null && t.memoizedState.cachePool !== null && (u = t.memoizedState.cachePool.pool), u !== a && (t.flags |= 2048), l !== null && T(Ca), null;
      case 24:
        return a = null, l !== null && (a = l.memoizedState.cache), t.memoizedState.cache !== a && (t.flags |= 2048), Lt(Ml), hl(t), null;
      case 25:
        return null;
      case 30:
        return null;
    }
    throw Error(m(156, t.tag));
  }
  function ly(l, t) {
    switch (Dc(t), t.tag) {
      case 1:
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 3:
        return Lt(Ml), Tl(), l = t.flags, (l & 65536) !== 0 && (l & 128) === 0 ? (t.flags = l & -65537 | 128, t) : null;
      case 26:
      case 27:
      case 5:
        return pe(t), null;
      case 31:
        if (t.memoizedState !== null) {
          if (dt(t), t.alternate === null)
            throw Error(m(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 13:
        if (dt(t), l = t.memoizedState, l !== null && l.dehydrated !== null) {
          if (t.alternate === null)
            throw Error(m(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 19:
        return T(El), null;
      case 4:
        return Tl(), null;
      case 10:
        return Lt(t.type), null;
      case 22:
      case 23:
        return dt(t), Lc(), l !== null && T(Ca), l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 24:
        return Lt(Ml), null;
      case 25:
        return null;
      default:
        return null;
    }
  }
  function Lv(l, t) {
    switch (Dc(t), t.tag) {
      case 3:
        Lt(Ml), Tl();
        break;
      case 26:
      case 27:
      case 5:
        pe(t);
        break;
      case 4:
        Tl();
        break;
      case 31:
        t.memoizedState !== null && dt(t);
        break;
      case 13:
        dt(t);
        break;
      case 19:
        T(El);
        break;
      case 10:
        Lt(t.type);
        break;
      case 22:
      case 23:
        dt(t), Lc(), l !== null && T(Ca);
        break;
      case 24:
        Lt(Ml);
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
              var f = a, y = i;
              try {
                y();
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
  function Vv(l) {
    var t = l.updateQueue;
    if (t !== null) {
      var a = l.stateNode;
      try {
        xs(t, a);
      } catch (u) {
        nl(l, l.return, u);
      }
    }
  }
  function Kv(l, t, a) {
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
  function Jv(l) {
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
      Ty(u, l.type, a, t), u[Wl] = t;
    } catch (e) {
      nl(l, l.return, e);
    }
  }
  function wv(l) {
    return l.tag === 5 || l.tag === 3 || l.tag === 26 || l.tag === 27 && _a(l.type) || l.tag === 4;
  }
  function Ti(l) {
    l: for (; ; ) {
      for (; l.sibling === null; ) {
        if (l.return === null || wv(l.return)) return null;
        l = l.return;
      }
      for (l.sibling.return = l.return, l = l.sibling; l.tag !== 5 && l.tag !== 6 && l.tag !== 18; ) {
        if (l.tag === 27 && _a(l.type) || l.flags & 2 || l.child === null || l.tag === 4) continue l;
        l.child.return = l, l = l.child;
      }
      if (!(l.flags & 2)) return l.stateNode;
    }
  }
  function Ei(l, t, a) {
    var u = l.tag;
    if (u === 5 || u === 6)
      l = l.stateNode, t ? (a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a).insertBefore(l, t) : (t = a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a, t.appendChild(l), a = a._reactRootContainer, a != null || t.onclick !== null || (t.onclick = Yt));
    else if (u !== 4 && (u === 27 && _a(l.type) && (a = l.stateNode, t = null), l = l.child, l !== null))
      for (Ei(l, t, a), l = l.sibling; l !== null; )
        Ei(l, t, a), l = l.sibling;
  }
  function gn(l, t, a) {
    var u = l.tag;
    if (u === 5 || u === 6)
      l = l.stateNode, t ? a.insertBefore(l, t) : a.appendChild(l);
    else if (u !== 4 && (u === 27 && _a(l.type) && (a = l.stateNode), l = l.child, l !== null))
      for (gn(l, t, a), l = l.sibling; l !== null; )
        gn(l, t, a), l = l.sibling;
  }
  function kv(l) {
    var t = l.stateNode, a = l.memoizedProps;
    try {
      for (var u = l.type, e = t.attributes; e.length; )
        t.removeAttributeNode(e[0]);
      Ll(t, u, a), t[Gl] = l, t[Wl] = a;
    } catch (n) {
      nl(l, l.return, n);
    }
  }
  var kt = !1, Ul = !1, Ai = !1, Wv = typeof WeakSet == "function" ? WeakSet : Set, Rl = null;
  function ty(l, t) {
    if (l = l.containerInfo, Ki = Cn, l = is(l), Sc(l)) {
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
            var c = 0, i = -1, f = -1, y = 0, r = 0, z = l, h = null;
            t: for (; ; ) {
              for (var g; z !== a || e !== 0 && z.nodeType !== 3 || (i = c + e), z !== n || u !== 0 && z.nodeType !== 3 || (f = c + u), z.nodeType === 3 && (c += z.nodeValue.length), (g = z.firstChild) !== null; )
                h = z, z = g;
              for (; ; ) {
                if (z === l) break t;
                if (h === a && ++y === e && (i = c), h === n && ++r === u && (f = c), (g = z.nextSibling) !== null) break;
                z = h, h = z.parentNode;
              }
              z = g;
            }
            a = i === -1 || f === -1 ? null : { start: i, end: f };
          } else a = null;
        }
      a = a || { start: 0, end: 0 };
    } else a = null;
    for (Ji = { focusedElem: l, selectionRange: a }, Cn = !1, Rl = t; Rl !== null; )
      if (t = Rl, l = t.child, (t.subtreeFlags & 1028) !== 0 && l !== null)
        l.return = t, Rl = l;
      else
        for (; Rl !== null; ) {
          switch (t = Rl, n = t.alternate, l = t.flags, t.tag) {
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
                  var U = Za(
                    a.type,
                    e
                  );
                  l = u.getSnapshotBeforeUpdate(
                    U,
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
              if ((l & 1024) !== 0) throw Error(m(163));
          }
          if (l = t.sibling, l !== null) {
            l.return = t.return, Rl = l;
            break;
          }
          Rl = t.return;
        }
  }
  function $v(l, t, a) {
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
        u & 64 && Vv(a), u & 512 && ce(a, a.return);
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
            xs(l, t);
          } catch (c) {
            nl(a, a.return, c);
          }
        }
        break;
      case 27:
        t === null && u & 4 && kv(a);
      case 26:
      case 5:
        $t(l, a), t === null && u & 4 && Jv(a), u & 512 && ce(a, a.return);
        break;
      case 12:
        $t(l, a);
        break;
      case 31:
        $t(l, a), u & 4 && Pv(l, a);
        break;
      case 13:
        $t(l, a), u & 4 && l0(l, a), u & 64 && (l = a.memoizedState, l !== null && (l = l.dehydrated, l !== null && (a = vy.bind(
          null,
          a
        ), Ny(l, a))));
        break;
      case 22:
        if (u = a.memoizedState !== null || kt, !u) {
          t = t !== null && t.memoizedState !== null || Ul, e = kt;
          var n = Ul;
          kt = u, (Ul = t) && !n ? Ft(
            l,
            a,
            (a.subtreeFlags & 8772) !== 0
          ) : $t(l, a), kt = e, Ul = n;
        }
        break;
      case 30:
        break;
      default:
        $t(l, a);
    }
  }
  function Fv(l) {
    var t = l.alternate;
    t !== null && (l.alternate = null, Fv(t)), l.child = null, l.deletions = null, l.sibling = null, l.tag === 5 && (t = l.stateNode, t !== null && lc(t)), l.stateNode = null, l.return = null, l.dependencies = null, l.memoizedProps = null, l.memoizedState = null, l.pendingProps = null, l.stateNode = null, l.updateQueue = null;
  }
  var Sl = null, Fl = !1;
  function Wt(l, t, a) {
    for (a = a.child; a !== null; )
      Iv(l, t, a), a = a.sibling;
  }
  function Iv(l, t, a) {
    if (ct && typeof ct.onCommitFiberUnmount == "function")
      try {
        ct.onCommitFiberUnmount(Nu, a);
      } catch {
      }
    switch (a.tag) {
      case 26:
        Ul || Rt(a, t), Wt(
          l,
          t,
          a
        ), a.memoizedState ? a.memoizedState.count-- : a.stateNode && (a = a.stateNode, a.parentNode.removeChild(a));
        break;
      case 27:
        Ul || Rt(a, t);
        var u = Sl, e = Fl;
        _a(a.type) && (Sl = a.stateNode, Fl = !1), Wt(
          l,
          t,
          a
        ), he(a.stateNode), Sl = u, Fl = e;
        break;
      case 5:
        Ul || Rt(a, t);
      case 6:
        if (u = Sl, e = Fl, Sl = null, Wt(
          l,
          t,
          a
        ), Sl = u, Fl = e, Sl !== null)
          if (Fl)
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
        Sl !== null && (Fl ? (l = Sl, V0(
          l.nodeType === 9 ? l.body : l.nodeName === "HTML" ? l.ownerDocument.body : l,
          a.stateNode
        ), Du(l)) : V0(Sl, a.stateNode));
        break;
      case 4:
        u = Sl, e = Fl, Sl = a.stateNode.containerInfo, Fl = !0, Wt(
          l,
          t,
          a
        ), Sl = u, Fl = e;
        break;
      case 0:
      case 11:
      case 14:
      case 15:
        ya(2, a, t), Ul || ya(4, a, t), Wt(
          l,
          t,
          a
        );
        break;
      case 1:
        Ul || (Rt(a, t), u = a.stateNode, typeof u.componentWillUnmount == "function" && Kv(
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
        Ul = (u = Ul) || a.memoizedState !== null, Wt(
          l,
          t,
          a
        ), Ul = u;
        break;
      default:
        Wt(
          l,
          t,
          a
        );
    }
  }
  function Pv(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null))) {
      l = l.dehydrated;
      try {
        Du(l);
      } catch (a) {
        nl(t, t.return, a);
      }
    }
  }
  function l0(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null && (l = l.dehydrated, l !== null))))
      try {
        Du(l);
      } catch (a) {
        nl(t, t.return, a);
      }
  }
  function ay(l) {
    switch (l.tag) {
      case 31:
      case 13:
      case 19:
        var t = l.stateNode;
        return t === null && (t = l.stateNode = new Wv()), t;
      case 22:
        return l = l.stateNode, t = l._retryCache, t === null && (t = l._retryCache = new Wv()), t;
      default:
        throw Error(m(435, l.tag));
    }
  }
  function Sn(l, t) {
    var a = ay(l);
    t.forEach(function(u) {
      if (!a.has(u)) {
        a.add(u);
        var e = dy.bind(null, l, u);
        u.then(e, e);
      }
    });
  }
  function Il(l, t) {
    var a = t.deletions;
    if (a !== null)
      for (var u = 0; u < a.length; u++) {
        var e = a[u], n = l, c = t, i = c;
        l: for (; i !== null; ) {
          switch (i.tag) {
            case 27:
              if (_a(i.type)) {
                Sl = i.stateNode, Fl = !1;
                break l;
              }
              break;
            case 5:
              Sl = i.stateNode, Fl = !1;
              break l;
            case 3:
            case 4:
              Sl = i.stateNode.containerInfo, Fl = !0;
              break l;
          }
          i = i.return;
        }
        if (Sl === null) throw Error(m(160));
        Iv(n, c, e), Sl = null, Fl = !1, n = e.alternate, n !== null && (n.return = null), e.return = null;
      }
    if (t.subtreeFlags & 13886)
      for (t = t.child; t !== null; )
        t0(t, l), t = t.sibling;
  }
  var Dt = null;
  function t0(l, t) {
    var a = l.alternate, u = l.flags;
    switch (l.tag) {
      case 0:
      case 11:
      case 14:
      case 15:
        Il(t, l), Pl(l), u & 4 && (ya(3, l, l.return), ne(3, l), ya(5, l, l.return));
        break;
      case 1:
        Il(t, l), Pl(l), u & 512 && (Ul || a === null || Rt(a, a.return)), u & 64 && kt && (l = l.updateQueue, l !== null && (u = l.callbacks, u !== null && (a = l.shared.hiddenCallbacks, l.shared.hiddenCallbacks = a === null ? u : a.concat(u))));
        break;
      case 26:
        var e = Dt;
        if (Il(t, l), Pl(l), u & 512 && (Ul || a === null || Rt(a, a.return)), u & 4) {
          var n = a !== null ? a.memoizedState : null;
          if (u = l.memoizedState, a === null)
            if (u === null)
              if (l.stateNode === null) {
                l: {
                  u = l.type, a = l.memoizedProps, e = e.ownerDocument || e;
                  t: switch (u) {
                    case "title":
                      n = e.getElementsByTagName("title")[0], (!n || n[Ru] || n[Gl] || n.namespaceURI === "http://www.w3.org/2000/svg" || n.hasAttribute("itemprop")) && (n = e.createElement(u), e.head.insertBefore(
                        n,
                        e.querySelector("head > title")
                      )), Ll(n, u, a), n[Gl] = l, Hl(n), u = n;
                      break l;
                    case "link":
                      var c = td(
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
                      n = e.createElement(u), Ll(n, u, a), e.head.appendChild(n);
                      break;
                    case "meta":
                      if (c = td(
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
                      n = e.createElement(u), Ll(n, u, a), e.head.appendChild(n);
                      break;
                    default:
                      throw Error(m(468, u));
                  }
                  n[Gl] = l, Hl(n), u = n;
                }
                l.stateNode = u;
              } else
                ad(
                  e,
                  l.type,
                  l.stateNode
                );
            else
              l.stateNode = ld(
                e,
                u,
                l.memoizedProps
              );
          else
            n !== u ? (n === null ? a.stateNode !== null && (a = a.stateNode, a.parentNode.removeChild(a)) : n.count--, u === null ? ad(
              e,
              l.type,
              l.stateNode
            ) : ld(
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
        Il(t, l), Pl(l), u & 512 && (Ul || a === null || Rt(a, a.return)), a !== null && u & 4 && zi(
          l,
          l.memoizedProps,
          a.memoizedProps
        );
        break;
      case 5:
        if (Il(t, l), Pl(l), u & 512 && (Ul || a === null || Rt(a, a.return)), l.flags & 32) {
          e = l.stateNode;
          try {
            Fa(e, "");
          } catch (U) {
            nl(l, l.return, U);
          }
        }
        u & 4 && l.stateNode != null && (e = l.memoizedProps, zi(
          l,
          e,
          a !== null ? a.memoizedProps : e
        )), u & 1024 && (Ai = !0);
        break;
      case 6:
        if (Il(t, l), Pl(l), u & 4) {
          if (l.stateNode === null)
            throw Error(m(162));
          u = l.memoizedProps, a = l.stateNode;
          try {
            a.nodeValue = u;
          } catch (U) {
            nl(l, l.return, U);
          }
        }
        break;
      case 3:
        if (Rn = null, e = Dt, Dt = jn(t.containerInfo), Il(t, l), Dt = e, Pl(l), u & 4 && a !== null && a.memoizedState.isDehydrated)
          try {
            Du(t.containerInfo);
          } catch (U) {
            nl(l, l.return, U);
          }
        Ai && (Ai = !1, a0(l));
        break;
      case 4:
        u = Dt, Dt = jn(
          l.stateNode.containerInfo
        ), Il(t, l), Pl(l), Dt = u;
        break;
      case 12:
        Il(t, l), Pl(l);
        break;
      case 31:
        Il(t, l), Pl(l), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 13:
        Il(t, l), Pl(l), l.child.flags & 8192 && l.memoizedState !== null != (a !== null && a.memoizedState !== null) && (bn = nt()), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 22:
        e = l.memoizedState !== null;
        var f = a !== null && a.memoizedState !== null, y = kt, r = Ul;
        if (kt = y || e, Ul = r || f, Il(t, l), Ul = r, kt = y, Pl(l), u & 8192)
          l: for (t = l.stateNode, t._visibility = e ? t._visibility & -2 : t._visibility | 1, e && (a === null || f || kt || Ul || La(l)), a = null, t = l; ; ) {
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
                } catch (U) {
                  nl(f, f.return, U);
                }
              }
            } else if (t.tag === 6) {
              if (a === null) {
                f = t;
                try {
                  f.stateNode.nodeValue = e ? "" : f.memoizedProps;
                } catch (U) {
                  nl(f, f.return, U);
                }
              }
            } else if (t.tag === 18) {
              if (a === null) {
                f = t;
                try {
                  var g = f.stateNode;
                  e ? K0(g, !0) : K0(f.stateNode, !1);
                } catch (U) {
                  nl(f, f.return, U);
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
        Il(t, l), Pl(l), u & 4 && (u = l.updateQueue, u !== null && (l.updateQueue = null, Sn(l, u)));
        break;
      case 30:
        break;
      case 21:
        break;
      default:
        Il(t, l), Pl(l);
    }
  }
  function Pl(l) {
    var t = l.flags;
    if (t & 2) {
      try {
        for (var a, u = l.return; u !== null; ) {
          if (wv(u)) {
            a = u;
            break;
          }
          u = u.return;
        }
        if (a == null) throw Error(m(160));
        switch (a.tag) {
          case 27:
            var e = a.stateNode, n = Ti(l);
            gn(l, n, e);
            break;
          case 5:
            var c = a.stateNode;
            a.flags & 32 && (Fa(c, ""), a.flags &= -33);
            var i = Ti(l);
            gn(l, i, c);
            break;
          case 3:
          case 4:
            var f = a.stateNode.containerInfo, y = Ti(l);
            Ei(
              l,
              y,
              f
            );
            break;
          default:
            throw Error(m(161));
        }
      } catch (r) {
        nl(l, l.return, r);
      }
      l.flags &= -3;
    }
    t & 4096 && (l.flags &= -4097);
  }
  function a0(l) {
    if (l.subtreeFlags & 1024)
      for (l = l.child; l !== null; ) {
        var t = l;
        a0(t), t.tag === 5 && t.flags & 1024 && t.stateNode.reset(), l = l.sibling;
      }
  }
  function $t(l, t) {
    if (t.subtreeFlags & 8772)
      for (t = t.child; t !== null; )
        $v(l, t.alternate, t), t = t.sibling;
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
          typeof a.componentWillUnmount == "function" && Kv(
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
            } catch (y) {
              nl(u, u.return, y);
            }
          if (u = n, e = u.updateQueue, e !== null) {
            var i = u.stateNode;
            try {
              var f = e.shared.hiddenCallbacks;
              if (f !== null)
                for (e.shared.hiddenCallbacks = null, e = 0; e < f.length; e++)
                  Rs(f[e], i);
            } catch (y) {
              nl(u, u.return, y);
            }
          }
          a && c & 64 && Vv(n), ce(n, n.return);
          break;
        case 27:
          kv(n);
        case 26:
        case 5:
          Ft(
            e,
            n,
            a
          ), a && u === null && c & 4 && Jv(n), ce(n, n.return);
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
          ), a && c & 4 && Pv(e, n);
          break;
        case 13:
          Ft(
            e,
            n,
            a
          ), a && c & 4 && l0(e, n);
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
        u0(
          l,
          t,
          a,
          u
        ), t = t.sibling;
  }
  function u0(l, t, a, u) {
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
      var n = l, c = t, i = a, f = u, y = c.flags;
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
          )), e && y & 2048 && pi(
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
          ), e && y & 2048 && Mi(c.alternate, c);
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
        e0(
          l,
          t,
          a
        ), l = l.sibling;
  }
  function e0(l, t, a) {
    switch (l.tag) {
      case 26:
        ru(
          l,
          t,
          a
        ), l.flags & fe && l.memoizedState !== null && Zy(
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
  function n0(l) {
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
          Rl = u, i0(
            u,
            l
          );
        }
      n0(l);
    }
    if (l.subtreeFlags & 10256)
      for (l = l.child; l !== null; )
        c0(l), l = l.sibling;
  }
  function c0(l) {
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
          Rl = u, i0(
            u,
            l
          );
        }
      n0(l);
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
  function i0(l, t) {
    for (; Rl !== null; ) {
      var a = Rl;
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
      if (u = a.child, u !== null) u.return = a, Rl = u;
      else
        l: for (a = l; Rl !== null; ) {
          u = Rl;
          var e = u.sibling, n = u.return;
          if (Fv(u), u === a) {
            Rl = null;
            break l;
          }
          if (e !== null) {
            e.return = n, Rl = e;
            break l;
          }
          Rl = n;
        }
    }
  }
  var uy = {
    getCacheForType: function(l) {
      var t = Ql(Ml), a = t.data.get(l);
      return a === void 0 && (a = l(), t.data.set(l, a)), a;
    },
    cacheSignal: function() {
      return Ql(Ml).controller.signal;
    }
  }, ey = typeof WeakMap == "function" ? WeakMap : Map, ll = 0, ol = null, K = null, k = 0, el = 0, ot = null, ma = !1, bu = !1, Oi = !1, It = 0, zl = 0, ha = 0, Va = 0, Di = 0, yt = 0, _u = 0, ve = null, lt = null, Ui = !1, bn = 0, f0 = 0, _n = 1 / 0, zn = null, ga = null, jl = 0, Sa = null, zu = null, Pt = 0, Ni = 0, ji = null, s0 = null, de = 0, Hi = null;
  function mt() {
    return (ll & 2) !== 0 && k !== 0 ? k & -k : S.T !== null ? Yi() : pf();
  }
  function v0() {
    if (yt === 0)
      if ((k & 536870912) === 0 || $) {
        var l = De;
        De <<= 1, (De & 3932160) === 0 && (De = 262144), yt = l;
      } else yt = 536870912;
    return l = vt.current, l !== null && (l.flags |= 32), yt;
  }
  function tt(l, t, a) {
    (l === ol && (el === 2 || el === 9) || l.cancelPendingCommit !== null) && (Tu(l, 0), ra(
      l,
      k,
      yt,
      !1
    )), Hu(l, a), ((ll & 2) === 0 || l !== ol) && (l === ol && ((ll & 2) === 0 && (Va |= a), zl === 4 && ra(
      l,
      k,
      yt,
      !1
    )), xt(l));
  }
  function d0(l, t, a) {
    if ((ll & 6) !== 0) throw Error(m(327));
    var u = !a && (t & 127) === 0 && (t & l.expiredLanes) === 0 || ju(l, t), e = u ? iy(l, t) : xi(l, t, !0), n = u;
    do {
      if (e === 0) {
        bu && !u && ra(l, t, 0, !1);
        break;
      } else {
        if (a = l.current.alternate, n && !ny(a)) {
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
              if (f && (Tu(i, c).flags |= 256), c = xi(
                i,
                c,
                !1
              ), c !== 2) {
                if (Oi && !f) {
                  i.errorRecoveryDisabledLanes |= n, Va |= n, e = 4;
                  break l;
                }
                n = lt, lt = e, n !== null && (lt === null ? lt = n : lt.push.apply(
                  lt,
                  n
                ));
              }
              e = c;
            }
            if (n = !1, e !== 2) continue;
          }
        }
        if (e === 1) {
          Tu(l, 0), ra(l, t, 0, !0);
          break;
        }
        l: {
          switch (u = l, n = e, n) {
            case 0:
            case 1:
              throw Error(m(345));
            case 4:
              if ((t & 4194048) !== t) break;
            case 6:
              ra(
                u,
                t,
                yt,
                !ma
              );
              break l;
            case 2:
              lt = null;
              break;
            case 3:
            case 5:
              break;
            default:
              throw Error(m(329));
          }
          if ((t & 62914560) === t && (e = bn + 300 - nt(), 10 < e)) {
            if (ra(
              u,
              t,
              yt,
              !ma
            ), Ne(u, 0, !0) !== 0) break l;
            Pt = t, u.timeoutHandle = Z0(
              o0.bind(
                null,
                u,
                a,
                lt,
                zn,
                Ui,
                t,
                yt,
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
          o0(
            u,
            a,
            lt,
            zn,
            Ui,
            t,
            yt,
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
  function o0(l, t, a, u, e, n, c, i, f, y, r, z, h, g) {
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
      }, e0(
        t,
        n,
        z
      );
      var U = (n & 62914560) === n ? bn - nt() : (n & 4194048) === n ? f0 - nt() : 0;
      if (U = Ly(
        z,
        U
      ), U !== null) {
        Pt = n, l.cancelPendingCommit = U(
          _0.bind(
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
        ), ra(l, n, c, !y);
        return;
      }
    }
    _0(
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
  function ny(l) {
    for (var t = l; ; ) {
      var a = t.tag;
      if ((a === 0 || a === 11 || a === 15) && t.flags & 16384 && (a = t.updateQueue, a !== null && (a = a.stores, a !== null)))
        for (var u = 0; u < a.length; u++) {
          var e = a[u], n = e.getSnapshot;
          e = e.value;
          try {
            if (!ft(n(), e)) return !1;
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
      var n = 31 - it(e), c = 1 << n;
      u[n] = -1, e &= ~c;
    }
    a !== 0 && Tf(l, a, t);
  }
  function Tn() {
    return (ll & 6) === 0 ? (oe(0), !1) : !0;
  }
  function Ri() {
    if (K !== null) {
      if (el === 0)
        var l = K.return;
      else
        l = K, Zt = qa = null, Wc(l), ou = null, ku = 0, l = K;
      for (; l !== null; )
        Lv(l.alternate, l), l = l.return;
      K = null;
    }
  }
  function Tu(l, t) {
    var a = l.timeoutHandle;
    a !== -1 && (l.timeoutHandle = -1, py(a)), a = l.cancelPendingCommit, a !== null && (l.cancelPendingCommit = null, a()), Pt = 0, Ri(), ol = l, K = a = Xt(l.current, null), k = t, el = 0, ot = null, ma = !1, bu = ju(l, t), Oi = !1, _u = yt = Di = Va = ha = zl = 0, lt = ve = null, Ui = !1, (t & 8) !== 0 && (t |= t & 32);
    var u = l.entangledLanes;
    if (u !== 0)
      for (l = l.entanglements, u &= t; 0 < u; ) {
        var e = 31 - it(u), n = 1 << e;
        t |= l[e], u &= ~n;
      }
    return It = t, Ze(), a;
  }
  function y0(l, t) {
    X = null, S.H = ae, t === du || t === $e ? (t = Us(), el = 3) : t === Cc ? (t = Us(), el = 4) : el = t === di ? 8 : t !== null && typeof t == "object" && typeof t.then == "function" ? 6 : 1, ot = t, K === null && (zl = 1, dn(
      l,
      bt(t, l.current)
    ));
  }
  function m0() {
    var l = vt.current;
    return l === null ? !0 : (k & 4194048) === k ? Et === null : (k & 62914560) === k || (k & 536870912) !== 0 ? l === Et : !1;
  }
  function h0() {
    var l = S.H;
    return S.H = ae, l === null ? ae : l;
  }
  function g0() {
    var l = S.A;
    return S.A = uy, l;
  }
  function En() {
    zl = 4, ma || (k & 4194048) !== k && vt.current !== null || (bu = !0), (ha & 134217727) === 0 && (Va & 134217727) === 0 || ol === null || ra(
      ol,
      k,
      yt,
      !1
    );
  }
  function xi(l, t, a) {
    var u = ll;
    ll |= 2;
    var e = h0(), n = g0();
    (ol !== l || k !== t) && (zn = null, Tu(l, t)), t = !1;
    var c = zl;
    l: do
      try {
        if (el !== 0 && K !== null) {
          var i = K, f = ot;
          switch (el) {
            case 8:
              Ri(), c = 6;
              break l;
            case 3:
            case 2:
            case 9:
            case 6:
              vt.current === null && (t = !0);
              var y = el;
              if (el = 0, ot = null, Eu(l, i, f, y), a && bu) {
                c = 0;
                break l;
              }
              break;
            default:
              y = el, el = 0, ot = null, Eu(l, i, f, y);
          }
        }
        cy(), c = zl;
        break;
      } catch (r) {
        y0(l, r);
      }
    while (!0);
    return t && l.shellSuspendCounter++, Zt = qa = null, ll = u, S.H = e, S.A = n, K === null && (ol = null, k = 0, Ze()), c;
  }
  function cy() {
    for (; K !== null; ) S0(K);
  }
  function iy(l, t) {
    var a = ll;
    ll |= 2;
    var u = h0(), e = g0();
    ol !== l || k !== t ? (zn = null, _n = nt() + 500, Tu(l, t)) : bu = ju(
      l,
      t
    );
    l: do
      try {
        if (el !== 0 && K !== null) {
          t = K;
          var n = ot;
          t: switch (el) {
            case 1:
              el = 0, ot = null, Eu(l, t, n, 1);
              break;
            case 2:
            case 9:
              if (Os(n)) {
                el = 0, ot = null, r0(t);
                break;
              }
              t = function() {
                el !== 2 && el !== 9 || ol !== l || (el = 7), xt(l);
              }, n.then(t, t);
              break l;
            case 3:
              el = 7;
              break l;
            case 4:
              el = 5;
              break l;
            case 7:
              Os(n) ? (el = 0, ot = null, r0(t)) : (el = 0, ot = null, Eu(l, t, n, 7));
              break;
            case 5:
              var c = null;
              switch (K.tag) {
                case 26:
                  c = K.memoizedState;
                case 5:
                case 27:
                  var i = K;
                  if (c ? ud(c) : i.stateNode.complete) {
                    el = 0, ot = null;
                    var f = i.sibling;
                    if (f !== null) K = f;
                    else {
                      var y = i.return;
                      y !== null ? (K = y, An(y)) : K = null;
                    }
                    break t;
                  }
              }
              el = 0, ot = null, Eu(l, t, n, 5);
              break;
            case 6:
              el = 0, ot = null, Eu(l, t, n, 6);
              break;
            case 8:
              Ri(), zl = 6;
              break l;
            default:
              throw Error(m(462));
          }
        }
        fy();
        break;
      } catch (r) {
        y0(l, r);
      }
    while (!0);
    return Zt = qa = null, S.H = u, S.A = e, ll = a, K !== null ? 0 : (ol = null, k = 0, Ze(), zl);
  }
  function fy() {
    for (; K !== null && !Nd(); )
      S0(K);
  }
  function S0(l) {
    var t = Qv(l.alternate, l, It);
    l.memoizedProps = l.pendingProps, t === null ? An(l) : K = t;
  }
  function r0(l) {
    var t = l, a = t.alternate;
    switch (t.tag) {
      case 15:
      case 0:
        t = qv(
          a,
          t,
          t.pendingProps,
          t.type,
          void 0,
          k
        );
        break;
      case 11:
        t = qv(
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
        Lv(a, t), t = K = gs(t, It), t = Qv(a, t, It);
    }
    l.memoizedProps = l.pendingProps, t === null ? An(l) : K = t;
  }
  function Eu(l, t, a, u) {
    Zt = qa = null, Wc(t), ou = null, ku = 0;
    var e = t.return;
    try {
      if ($o(
        l,
        e,
        t,
        a,
        k
      )) {
        zl = 1, dn(
          l,
          bt(a, l.current)
        ), K = null;
        return;
      }
    } catch (n) {
      if (e !== null) throw K = e, n;
      zl = 1, dn(
        l,
        bt(a, l.current)
      ), K = null;
      return;
    }
    t.flags & 32768 ? ($ || u === 1 ? l = !0 : bu || (k & 536870912) !== 0 ? l = !1 : (ma = l = !0, (u === 2 || u === 9 || u === 3 || u === 6) && (u = vt.current, u !== null && u.tag === 13 && (u.flags |= 16384))), b0(t, l)) : An(t);
  }
  function An(l) {
    var t = l;
    do {
      if ((t.flags & 32768) !== 0) {
        b0(
          t,
          ma
        );
        return;
      }
      l = t.return;
      var a = Po(
        t.alternate,
        t,
        It
      );
      if (a !== null) {
        K = a;
        return;
      }
      if (t = t.sibling, t !== null) {
        K = t;
        return;
      }
      K = t = l;
    } while (t !== null);
    zl === 0 && (zl = 5);
  }
  function b0(l, t) {
    do {
      var a = ly(l.alternate, l);
      if (a !== null) {
        a.flags &= 32767, K = a;
        return;
      }
      if (a = l.return, a !== null && (a.flags |= 32768, a.subtreeFlags = 0, a.deletions = null), !t && (l = l.sibling, l !== null)) {
        K = l;
        return;
      }
      K = l = a;
    } while (l !== null);
    zl = 6, K = null;
  }
  function _0(l, t, a, u, e, n, c, i, f) {
    l.cancelPendingCommit = null;
    do
      pn();
    while (jl !== 0);
    if ((ll & 6) !== 0) throw Error(m(327));
    if (t !== null) {
      if (t === l.current) throw Error(m(177));
      if (n = t.lanes | t.childLanes, n |= Tc, Xd(
        l,
        a,
        n,
        c,
        i,
        f
      ), l === ol && (K = ol = null, k = 0), zu = t, Sa = l, Pt = a, Ni = n, ji = e, s0 = u, (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? (l.callbackNode = null, l.callbackPriority = 0, oy(Me, function() {
        return p0(), null;
      })) : (l.callbackNode = null, l.callbackPriority = 0), u = (t.flags & 13878) !== 0, (t.subtreeFlags & 13878) !== 0 || u) {
        u = S.T, S.T = null, e = M.p, M.p = 2, c = ll, ll |= 4;
        try {
          ty(l, t, a);
        } finally {
          ll = c, M.p = e, S.T = u;
        }
      }
      jl = 1, z0(), T0(), E0();
    }
  }
  function z0() {
    if (jl === 1) {
      jl = 0;
      var l = Sa, t = zu, a = (t.flags & 13878) !== 0;
      if ((t.subtreeFlags & 13878) !== 0 || a) {
        a = S.T, S.T = null;
        var u = M.p;
        M.p = 2;
        var e = ll;
        ll |= 4;
        try {
          t0(t, l);
          var n = Ji, c = is(l.containerInfo), i = n.focusedElem, f = n.selectionRange;
          if (c !== i && i && i.ownerDocument && cs(
            i.ownerDocument.documentElement,
            i
          )) {
            if (f !== null && Sc(i)) {
              var y = f.start, r = f.end;
              if (r === void 0 && (r = y), "selectionStart" in i)
                i.selectionStart = y, i.selectionEnd = Math.min(
                  r,
                  i.value.length
                );
              else {
                var z = i.ownerDocument || document, h = z && z.defaultView || window;
                if (h.getSelection) {
                  var g = h.getSelection(), U = i.textContent.length, B = Math.min(f.start, U), vl = f.end === void 0 ? B : Math.min(f.end, U);
                  !g.extend && B > vl && (c = vl, vl = B, B = c);
                  var d = ns(
                    i,
                    B
                  ), s = ns(
                    i,
                    vl
                  );
                  if (d && s && (g.rangeCount !== 1 || g.anchorNode !== d.node || g.anchorOffset !== d.offset || g.focusNode !== s.node || g.focusOffset !== s.offset)) {
                    var o = z.createRange();
                    o.setStart(d.node, d.offset), g.removeAllRanges(), B > vl ? (g.addRange(o), g.extend(s.node, s.offset)) : (o.setEnd(s.node, s.offset), g.addRange(o));
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
          ll = e, M.p = u, S.T = a;
        }
      }
      l.current = t, jl = 2;
    }
  }
  function T0() {
    if (jl === 2) {
      jl = 0;
      var l = Sa, t = zu, a = (t.flags & 8772) !== 0;
      if ((t.subtreeFlags & 8772) !== 0 || a) {
        a = S.T, S.T = null;
        var u = M.p;
        M.p = 2;
        var e = ll;
        ll |= 4;
        try {
          $v(l, t.alternate, t);
        } finally {
          ll = e, M.p = u, S.T = a;
        }
      }
      jl = 3;
    }
  }
  function E0() {
    if (jl === 4 || jl === 3) {
      jl = 0, jd();
      var l = Sa, t = zu, a = Pt, u = s0;
      (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? jl = 5 : (jl = 0, zu = Sa = null, A0(l, l.pendingLanes));
      var e = l.pendingLanes;
      if (e === 0 && (ga = null), In(a), t = t.stateNode, ct && typeof ct.onCommitFiberRoot == "function")
        try {
          ct.onCommitFiberRoot(
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
      (Pt & 3) !== 0 && pn(), xt(l), e = l.pendingLanes, (a & 261930) !== 0 && (e & 42) !== 0 ? l === Hi ? de++ : (de = 0, Hi = l) : de = 0, oe(0);
    }
  }
  function A0(l, t) {
    (l.pooledCacheLanes &= t) === 0 && (t = l.pooledCache, t != null && (l.pooledCache = null, Ju(t)));
  }
  function pn() {
    return z0(), T0(), E0(), p0();
  }
  function p0() {
    if (jl !== 5) return !1;
    var l = Sa, t = Ni;
    Ni = 0;
    var a = In(Pt), u = S.T, e = M.p;
    try {
      M.p = 32 > a ? 32 : a, S.T = null, a = ji, ji = null;
      var n = Sa, c = Pt;
      if (jl = 0, zu = Sa = null, Pt = 0, (ll & 6) !== 0) throw Error(m(331));
      var i = ll;
      if (ll |= 4, c0(n.current), u0(
        n,
        n.current,
        c,
        a
      ), ll = i, oe(0, !1), ct && typeof ct.onPostCommitFiberRoot == "function")
        try {
          ct.onPostCommitFiberRoot(Nu, n);
        } catch {
        }
      return !0;
    } finally {
      M.p = e, S.T = u, A0(l, t);
    }
  }
  function M0(l, t, a) {
    t = bt(a, t), t = vi(l.stateNode, t, 2), l = va(l, t, 2), l !== null && (Hu(l, 2), xt(l));
  }
  function nl(l, t, a) {
    if (l.tag === 3)
      M0(l, l, a);
    else
      for (; t !== null; ) {
        if (t.tag === 3) {
          M0(
            t,
            l,
            a
          );
          break;
        } else if (t.tag === 1) {
          var u = t.stateNode;
          if (typeof t.type.getDerivedStateFromError == "function" || typeof u.componentDidCatch == "function" && (ga === null || !ga.has(u))) {
            l = bt(a, l), a = Ov(2), u = va(t, a, 2), u !== null && (Dv(
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
      u = l.pingCache = new ey();
      var e = /* @__PURE__ */ new Set();
      u.set(t, e);
    } else
      e = u.get(t), e === void 0 && (e = /* @__PURE__ */ new Set(), u.set(t, e));
    e.has(a) || (Oi = !0, e.add(a), l = sy.bind(null, l, t, a), t.then(l, l));
  }
  function sy(l, t, a) {
    var u = l.pingCache;
    u !== null && u.delete(t), l.pingedLanes |= l.suspendedLanes & a, l.warmLanes &= ~a, ol === l && (k & a) === a && (zl === 4 || zl === 3 && (k & 62914560) === k && 300 > nt() - bn ? (ll & 2) === 0 && Tu(l, 0) : Di |= a, _u === k && (_u = 0)), xt(l);
  }
  function O0(l, t) {
    t === 0 && (t = zf()), l = Ha(l, t), l !== null && (Hu(l, t), xt(l));
  }
  function vy(l) {
    var t = l.memoizedState, a = 0;
    t !== null && (a = t.retryLane), O0(l, a);
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
        throw Error(m(314));
    }
    u !== null && u.delete(t), O0(l, a);
  }
  function oy(l, t) {
    return kn(l, t);
  }
  var Mn = null, Au = null, Bi = !1, On = !1, Ci = !1, ba = 0;
  function xt(l) {
    l !== Au && l.next === null && (Au === null ? Mn = Au = l : Au = Au.next = l), On = !0, Bi || (Bi = !0, my());
  }
  function oe(l, t) {
    if (!Ci && On) {
      Ci = !0;
      do
        for (var a = !1, u = Mn; u !== null; ) {
          if (l !== 0) {
            var e = u.pendingLanes;
            if (e === 0) var n = 0;
            else {
              var c = u.suspendedLanes, i = u.pingedLanes;
              n = (1 << 31 - it(42 | l) + 1) - 1, n &= e & ~(c & ~i), n = n & 201326741 ? n & 201326741 | 1 : n ? n | 2 : 0;
            }
            n !== 0 && (a = !0, j0(u, n));
          } else
            n = k, n = Ne(
              u,
              u === ol ? n : 0,
              u.cancelPendingCommit !== null || u.timeoutHandle !== -1
            ), (n & 3) === 0 || ju(u, n) || (a = !0, j0(u, n));
          u = u.next;
        }
      while (a);
      Ci = !1;
    }
  }
  function yy() {
    D0();
  }
  function D0() {
    On = Bi = !1;
    var l = 0;
    ba !== 0 && Ay() && (l = ba);
    for (var t = nt(), a = null, u = Mn; u !== null; ) {
      var e = u.next, n = U0(u, t);
      n === 0 ? (u.next = null, a === null ? Mn = e : a.next = e, e === null && (Au = a)) : (a = u, (l !== 0 || (n & 3) !== 0) && (On = !0)), u = e;
    }
    jl !== 0 && jl !== 5 || oe(l), ba !== 0 && (ba = 0);
  }
  function U0(l, t) {
    for (var a = l.suspendedLanes, u = l.pingedLanes, e = l.expirationTimes, n = l.pendingLanes & -62914561; 0 < n; ) {
      var c = 31 - it(n), i = 1 << c, f = e[c];
      f === -1 ? ((i & a) === 0 || (i & u) !== 0) && (e[c] = Gd(i, t)) : f <= t && (l.expiredLanes |= i), n &= ~i;
    }
    if (t = ol, a = k, a = Ne(
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
          a = bf;
          break;
        case 32:
          a = Me;
          break;
        case 268435456:
          a = _f;
          break;
        default:
          a = Me;
      }
      return u = N0.bind(null, l), a = kn(a, u), l.callbackPriority = t, l.callbackNode = a, t;
    }
    return u !== null && u !== null && Wn(u), l.callbackPriority = 2, l.callbackNode = null, 2;
  }
  function N0(l, t) {
    if (jl !== 0 && jl !== 5)
      return l.callbackNode = null, l.callbackPriority = 0, null;
    var a = l.callbackNode;
    if (pn() && l.callbackNode !== a)
      return null;
    var u = k;
    return u = Ne(
      l,
      l === ol ? u : 0,
      l.cancelPendingCommit !== null || l.timeoutHandle !== -1
    ), u === 0 ? null : (d0(l, u, t), U0(l, nt()), l.callbackNode != null && l.callbackNode === a ? N0.bind(null, l) : null);
  }
  function j0(l, t) {
    if (pn()) return null;
    d0(l, t, !0);
  }
  function my() {
    My(function() {
      (ll & 6) !== 0 ? kn(
        rf,
        yy
      ) : D0();
    });
  }
  function Yi() {
    if (ba === 0) {
      var l = su;
      l === 0 && (l = Oe, Oe <<= 1, (Oe & 261888) === 0 && (Oe = 256)), ba = l;
    }
    return ba;
  }
  function H0(l) {
    return l == null || typeof l == "symbol" || typeof l == "boolean" ? null : typeof l == "function" ? l : xe("" + l);
  }
  function R0(l, t) {
    var a = t.ownerDocument.createElement("input");
    return a.name = t.name, a.value = t.value, l.id && a.setAttribute("form", l.id), t.parentNode.insertBefore(a, t), l = new FormData(l), a.parentNode.removeChild(a), l;
  }
  function hy(l, t, a, u, e) {
    if (t === "submit" && a && a.stateNode === e) {
      var n = H0(
        (e[Wl] || null).action
      ), c = u.submitter;
      c && (t = (t = c[Wl] || null) ? H0(t.formAction) : c.getAttribute("formAction"), t !== null && (n = t, c = null));
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
                  var f = c ? R0(e, c) : new FormData(e);
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
                typeof n == "function" && (i.preventDefault(), f = c ? R0(e, c) : new FormData(e), ei(
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
    var Xi = zc[Gi], gy = Xi.toLowerCase(), Sy = Xi[0].toUpperCase() + Xi.slice(1);
    Ot(
      gy,
      "on" + Sy
    );
  }
  Ot(vs, "onAnimationEnd"), Ot(ds, "onAnimationIteration"), Ot(os, "onAnimationStart"), Ot("dblclick", "onDoubleClick"), Ot("focusin", "onFocus"), Ot("focusout", "onBlur"), Ot(Ro, "onTransitionRun"), Ot(xo, "onTransitionStart"), Ot(qo, "onTransitionCancel"), Ot(ys, "onTransitionEnd"), Wa("onMouseEnter", ["mouseout", "mouseover"]), Wa("onMouseLeave", ["mouseout", "mouseover"]), Wa("onPointerEnter", ["pointerout", "pointerover"]), Wa("onPointerLeave", ["pointerout", "pointerover"]), Da(
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
  ), ry = new Set(
    "beforetoggle cancel close invalid load scroll scrollend toggle".split(" ").concat(ye)
  );
  function x0(l, t) {
    t = (t & 4) !== 0;
    for (var a = 0; a < l.length; a++) {
      var u = l[a], e = u.event;
      u = u.listeners;
      l: {
        var n = void 0;
        if (t)
          for (var c = u.length - 1; 0 <= c; c--) {
            var i = u[c], f = i.instance, y = i.currentTarget;
            if (i = i.listener, f !== n && e.isPropagationStopped())
              break l;
            n = i, e.currentTarget = y;
            try {
              n(e);
            } catch (r) {
              Qe(r);
            }
            e.currentTarget = null, n = f;
          }
        else
          for (c = 0; c < u.length; c++) {
            if (i = u[c], f = i.instance, y = i.currentTarget, i = i.listener, f !== n && e.isPropagationStopped())
              break l;
            n = i, e.currentTarget = y;
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
  function J(l, t) {
    var a = t[Pn];
    a === void 0 && (a = t[Pn] = /* @__PURE__ */ new Set());
    var u = l + "__bubble";
    a.has(u) || (q0(t, l, 2, !1), a.add(u));
  }
  function Qi(l, t, a) {
    var u = 0;
    t && (u |= 4), q0(
      a,
      l,
      u,
      t
    );
  }
  var Dn = "_reactListening" + Math.random().toString(36).slice(2);
  function Zi(l) {
    if (!l[Dn]) {
      l[Dn] = !0, Df.forEach(function(a) {
        a !== "selectionchange" && (ry.has(a) || Qi(a, !1, l), Qi(a, !0, l));
      });
      var t = l.nodeType === 9 ? l : l.ownerDocument;
      t === null || t[Dn] || (t[Dn] = !0, Qi("selectionchange", !1, t));
    }
  }
  function q0(l, t, a, u) {
    switch (vd(t)) {
      case 2:
        var e = Jy;
        break;
      case 8:
        e = wy;
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
    Xf(function() {
      var y = n, r = cc(a), z = [];
      l: {
        var h = ms.get(l);
        if (h !== void 0) {
          var g = Ye, U = l;
          switch (l) {
            case "keypress":
              if (Be(a) === 0) break l;
            case "keydown":
            case "keyup":
              g = vo;
              break;
            case "focusin":
              U = "focus", g = oc;
              break;
            case "focusout":
              U = "blur", g = oc;
              break;
            case "beforeblur":
            case "afterblur":
              g = oc;
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
              g = Lf;
              break;
            case "drag":
            case "dragend":
            case "dragenter":
            case "dragexit":
            case "dragleave":
            case "dragover":
            case "dragstart":
            case "drop":
              g = Id;
              break;
            case "touchcancel":
            case "touchend":
            case "touchmove":
            case "touchstart":
              g = mo;
              break;
            case vs:
            case ds:
            case os:
              g = to;
              break;
            case ys:
              g = go;
              break;
            case "scroll":
            case "scrollend":
              g = $d;
              break;
            case "wheel":
              g = ro;
              break;
            case "copy":
            case "cut":
            case "paste":
              g = uo;
              break;
            case "gotpointercapture":
            case "lostpointercapture":
            case "pointercancel":
            case "pointerdown":
            case "pointermove":
            case "pointerout":
            case "pointerover":
            case "pointerup":
              g = Kf;
              break;
            case "toggle":
            case "beforetoggle":
              g = _o;
          }
          var B = (t & 4) !== 0, vl = !B && (l === "scroll" || l === "scrollend"), d = B ? h !== null ? h + "Capture" : null : h;
          B = [];
          for (var s = y, o; s !== null; ) {
            var b = s;
            if (o = b.stateNode, b = b.tag, b !== 5 && b !== 26 && b !== 27 || o === null || d === null || (b = qu(s, d), b != null && B.push(
              me(s, b, o)
            )), vl) break;
            s = s.return;
          }
          0 < B.length && (h = new g(
            h,
            U,
            null,
            a,
            r
          ), z.push({ event: h, listeners: B }));
        }
      }
      if ((t & 7) === 0) {
        l: {
          if (h = l === "mouseover" || l === "pointerover", g = l === "mouseout" || l === "pointerout", h && a !== nc && (U = a.relatedTarget || a.fromElement) && (Ja(U) || U[Ka]))
            break l;
          if ((g || h) && (h = r.window === r ? r : (h = r.ownerDocument) ? h.defaultView || h.parentWindow : window, g ? (U = a.relatedTarget || a.toElement, g = y, U = U ? Ja(U) : null, U !== null && (vl = yl(U), B = U.tag, U !== vl || B !== 5 && B !== 27 && B !== 6) && (U = null)) : (g = null, U = y), g !== U)) {
            if (B = Lf, b = "onMouseLeave", d = "onMouseEnter", s = "mouse", (l === "pointerout" || l === "pointerover") && (B = Kf, b = "onPointerLeave", d = "onPointerEnter", s = "pointer"), vl = g == null ? h : xu(g), o = U == null ? h : xu(U), h = new B(
              b,
              s + "leave",
              g,
              a,
              r
            ), h.target = vl, h.relatedTarget = o, b = null, Ja(r) === y && (B = new B(
              d,
              s + "enter",
              U,
              a,
              r
            ), B.target = o, B.relatedTarget = vl, b = B), vl = b, g && U)
              t: {
                for (B = by, d = g, s = U, o = 0, b = d; b; b = B(b))
                  o++;
                b = 0;
                for (var x = s; x; x = B(x))
                  b++;
                for (; 0 < o - b; )
                  d = B(d), o--;
                for (; 0 < b - o; )
                  s = B(s), b--;
                for (; o--; ) {
                  if (d === s || s !== null && d === s.alternate) {
                    B = d;
                    break t;
                  }
                  d = B(d), s = B(s);
                }
                B = null;
              }
            else B = null;
            g !== null && B0(
              z,
              h,
              g,
              B,
              !1
            ), U !== null && vl !== null && B0(
              z,
              vl,
              U,
              B,
              !0
            );
          }
        }
        l: {
          if (h = y ? xu(y) : window, g = h.nodeName && h.nodeName.toLowerCase(), g === "select" || g === "input" && h.type === "file")
            var F = Pf;
          else if (Ff(h))
            if (ls)
              F = No;
            else {
              F = Do;
              var R = Oo;
            }
          else
            g = h.nodeName, !g || g.toLowerCase() !== "input" || h.type !== "checkbox" && h.type !== "radio" ? y && ec(y.elementType) && (F = Pf) : F = Uo;
          if (F && (F = F(l, y))) {
            If(
              z,
              F,
              a,
              r
            );
            break l;
          }
          R && R(l, h, y), l === "focusout" && y && h.type === "number" && y.memoizedProps.value != null && uc(h, "number", h.value);
        }
        switch (R = y ? xu(y) : window, l) {
          case "focusin":
            (Ff(R) || R.contentEditable === "true") && (tu = R, rc = y, Lu = null);
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
            bc = !1, fs(z, a, r);
            break;
          case "selectionchange":
            if (Ho) break;
          case "keydown":
          case "keyup":
            fs(z, a, r);
        }
        var Z;
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
          lu ? Wf(l, a) && (W = "onCompositionEnd") : l === "keydown" && a.keyCode === 229 && (W = "onCompositionStart");
        W && (Jf && a.locale !== "ko" && (lu || W !== "onCompositionStart" ? W === "onCompositionEnd" && lu && (Z = Qf()) : (ua = r, sc = "value" in ua ? ua.value : ua.textContent, lu = !0)), R = Un(y, W), 0 < R.length && (W = new Vf(
          W,
          l,
          null,
          a,
          r
        ), z.push({ event: W, listeners: R }), Z ? W.data = Z : (Z = $f(a), Z !== null && (W.data = Z)))), (Z = To ? Eo(l, a) : Ao(l, a)) && (W = Un(y, "onBeforeInput"), 0 < W.length && (R = new Vf(
          "onBeforeInput",
          "beforeinput",
          null,
          a,
          r
        ), z.push({
          event: R,
          listeners: W
        }), R.data = Z)), hy(
          z,
          l,
          y,
          a,
          r
        );
      }
      x0(z, t);
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
  function by(l) {
    if (l === null) return null;
    do
      l = l.return;
    while (l && l.tag !== 5 && l.tag !== 27);
    return l || null;
  }
  function B0(l, t, a, u, e) {
    for (var n = t._reactName, c = []; a !== null && a !== u; ) {
      var i = a, f = i.alternate, y = i.stateNode;
      if (i = i.tag, f !== null && f === u) break;
      i !== 5 && i !== 26 && i !== 27 || y === null || (f = y, e ? (y = qu(a, n), y != null && c.unshift(
        me(a, y, f)
      )) : e || (y = qu(a, n), y != null && c.push(
        me(a, y, f)
      ))), a = a.return;
    }
    c.length !== 0 && l.push({ event: t, listeners: c });
  }
  var _y = /\r\n?/g, zy = /\u0000|\uFFFD/g;
  function C0(l) {
    return (typeof l == "string" ? l : "" + l).replace(_y, `
`).replace(zy, "");
  }
  function Y0(l, t) {
    return t = C0(t), C0(l) === t;
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
        Yf(l, u, n);
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
        u != null && J("scroll", l);
        break;
      case "onScrollEnd":
        u != null && J("scrollend", l);
        break;
      case "dangerouslySetInnerHTML":
        if (u != null) {
          if (typeof u != "object" || !("__html" in u))
            throw Error(m(61));
          if (a = u.__html, a != null) {
            if (e.children != null) throw Error(m(60));
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
        J("beforetoggle", l), J("toggle", l), je(l, "popover", u);
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
        (!(2 < a.length) || a[0] !== "o" && a[0] !== "O" || a[1] !== "n" && a[1] !== "N") && (a = kd.get(a) || a, je(l, a, u));
    }
  }
  function Vi(l, t, a, u, e, n) {
    switch (a) {
      case "style":
        Yf(l, u, n);
        break;
      case "dangerouslySetInnerHTML":
        if (u != null) {
          if (typeof u != "object" || !("__html" in u))
            throw Error(m(61));
          if (a = u.__html, a != null) {
            if (e.children != null) throw Error(m(60));
            l.innerHTML = a;
          }
        }
        break;
      case "children":
        typeof u == "string" ? Fa(l, u) : (typeof u == "number" || typeof u == "bigint") && Fa(l, "" + u);
        break;
      case "onScroll":
        u != null && J("scroll", l);
        break;
      case "onScrollEnd":
        u != null && J("scrollend", l);
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
        if (!Uf.hasOwnProperty(a))
          l: {
            if (a[0] === "o" && a[1] === "n" && (e = a.endsWith("Capture"), t = a.slice(2, e ? a.length - 7 : void 0), n = l[Wl] || null, n = n != null ? n[a] : null, typeof n == "function" && l.removeEventListener(t, n, e), typeof u == "function")) {
              typeof n != "function" && n !== null && (a in l ? l[a] = null : l.hasAttribute(a) && l.removeAttribute(a)), l.addEventListener(t, u, e);
              break l;
            }
            a in l ? l[a] = u : u === !0 ? l.setAttribute(a, "") : je(l, a, u);
          }
    }
  }
  function Ll(l, t, a) {
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
        J("error", l), J("load", l);
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
                  throw Error(m(137, t));
                default:
                  sl(l, t, n, c, a, null);
              }
          }
        e && sl(l, t, "srcSet", a.srcSet, a, null), u && sl(l, t, "src", a.src, a, null);
        return;
      case "input":
        J("invalid", l);
        var i = n = c = e = null, f = null, y = null;
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
                  y = r;
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
                    throw Error(m(137, t));
                  break;
                default:
                  sl(l, t, u, r, a, null);
              }
          }
        xf(
          l,
          n,
          i,
          f,
          y,
          c,
          e,
          !1
        );
        return;
      case "select":
        J("invalid", l), u = c = n = null;
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
        J("invalid", l), n = e = u = null;
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
                if (i != null) throw Error(m(91));
                break;
              default:
                sl(l, t, c, i, a, null);
            }
        Bf(l, u, e, n);
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
        J("beforetoggle", l), J("toggle", l), J("cancel", l), J("close", l);
        break;
      case "iframe":
      case "object":
        J("load", l);
        break;
      case "video":
      case "audio":
        for (u = 0; u < ye.length; u++)
          J(ye[u], l);
        break;
      case "image":
        J("error", l), J("load", l);
        break;
      case "details":
        J("toggle", l);
        break;
      case "embed":
      case "source":
      case "link":
        J("error", l), J("load", l);
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
        for (y in a)
          if (a.hasOwnProperty(y) && (u = a[y], u != null))
            switch (y) {
              case "children":
              case "dangerouslySetInnerHTML":
                throw Error(m(137, t));
              default:
                sl(l, t, y, u, a, null);
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
        var e = null, n = null, c = null, i = null, f = null, y = null, r = null;
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
                y = g;
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
                  throw Error(m(137, t));
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
          y,
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
                if (e != null) throw Error(m(91));
                break;
              default:
                e !== n && sl(l, t, c, e, u, n);
            }
        qf(l, h, g);
        return;
      case "option":
        for (var U in a)
          if (h = a[U], a.hasOwnProperty(U) && h != null && !u.hasOwnProperty(U))
            switch (U) {
              case "selected":
                l.selected = !1;
                break;
              default:
                sl(
                  l,
                  t,
                  U,
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
        for (y in u)
          if (h = u[y], g = a[y], u.hasOwnProperty(y) && h !== g && (h != null || g != null))
            switch (y) {
              case "children":
              case "dangerouslySetInnerHTML":
                if (h != null)
                  throw Error(m(137, t));
                break;
              default:
                sl(
                  l,
                  t,
                  y,
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
    for (var d in a)
      h = a[d], a.hasOwnProperty(d) && h != null && !u.hasOwnProperty(d) && sl(l, t, d, null, u, h);
    for (z in u)
      h = u[z], g = a[z], !u.hasOwnProperty(z) || h === g || h == null && g == null || sl(l, t, z, h, u, g);
  }
  function G0(l) {
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
  function Ey() {
    if (typeof performance.getEntriesByType == "function") {
      for (var l = 0, t = 0, a = performance.getEntriesByType("resource"), u = 0; u < a.length; u++) {
        var e = a[u], n = e.transferSize, c = e.initiatorType, i = e.duration;
        if (n && i && G0(c)) {
          for (c = 0, i = e.responseEnd, u += 1; u < a.length; u++) {
            var f = a[u], y = f.startTime;
            if (y > i) break;
            var r = f.transferSize, z = f.initiatorType;
            r && G0(z) && (f = f.responseEnd, c += r * (f < i ? 1 : (i - y) / (f - y)));
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
  function X0(l) {
    switch (l) {
      case "http://www.w3.org/2000/svg":
        return 1;
      case "http://www.w3.org/1998/Math/MathML":
        return 2;
      default:
        return 0;
    }
  }
  function Q0(l, t) {
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
  function Ay() {
    var l = window.event;
    return l && l.type === "popstate" ? l === ki ? !1 : (ki = l, !0) : (ki = null, !1);
  }
  var Z0 = typeof setTimeout == "function" ? setTimeout : void 0, py = typeof clearTimeout == "function" ? clearTimeout : void 0, L0 = typeof Promise == "function" ? Promise : void 0, My = typeof queueMicrotask == "function" ? queueMicrotask : typeof L0 < "u" ? function(l) {
    return L0.resolve(null).then(l).catch(Oy);
  } : Z0;
  function Oy(l) {
    setTimeout(function() {
      throw l;
    });
  }
  function _a(l) {
    return l === "head";
  }
  function V0(l, t) {
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
  function K0(l, t) {
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
  function Dy(l, t, a, u) {
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
      if (l = At(l.nextSibling), l === null) break;
    }
    return null;
  }
  function Uy(l, t, a) {
    if (t === "") return null;
    for (; l.nodeType !== 3; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !a || (l = At(l.nextSibling), l === null)) return null;
    return l;
  }
  function J0(l, t) {
    for (; l.nodeType !== 8; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !t || (l = At(l.nextSibling), l === null)) return null;
    return l;
  }
  function $i(l) {
    return l.data === "$?" || l.data === "$~";
  }
  function Fi(l) {
    return l.data === "$!" || l.data === "$?" && l.ownerDocument.readyState !== "loading";
  }
  function Ny(l, t) {
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
  function At(l) {
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
  function w0(l) {
    l = l.nextSibling;
    for (var t = 0; l; ) {
      if (l.nodeType === 8) {
        var a = l.data;
        if (a === "/$" || a === "/&") {
          if (t === 0)
            return At(l.nextSibling);
          t--;
        } else
          a !== "$" && a !== "$!" && a !== "$?" && a !== "$~" && a !== "&" || t++;
      }
      l = l.nextSibling;
    }
    return null;
  }
  function k0(l) {
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
  function W0(l, t, a) {
    switch (t = Nn(a), l) {
      case "html":
        if (l = t.documentElement, !l) throw Error(m(452));
        return l;
      case "head":
        if (l = t.head, !l) throw Error(m(453));
        return l;
      case "body":
        if (l = t.body, !l) throw Error(m(454));
        return l;
      default:
        throw Error(m(451));
    }
  }
  function he(l) {
    for (var t = l.attributes; t.length; )
      l.removeAttributeNode(t[0]);
    lc(l);
  }
  var pt = /* @__PURE__ */ new Map(), $0 = /* @__PURE__ */ new Set();
  function jn(l) {
    return typeof l.getRootNode == "function" ? l.getRootNode() : l.nodeType === 9 ? l : l.ownerDocument;
  }
  var la = M.d;
  M.d = {
    f: jy,
    r: Hy,
    D: Ry,
    C: xy,
    L: qy,
    m: By,
    X: Yy,
    S: Cy,
    M: Gy
  };
  function jy() {
    var l = la.f(), t = Tn();
    return l || t;
  }
  function Hy(l) {
    var t = wa(l);
    t !== null && t.tag === 5 && t.type === "form" ? yv(t) : la.r(l);
  }
  var pu = typeof document > "u" ? null : document;
  function F0(l, t, a) {
    var u = pu;
    if (u && typeof t == "string" && t) {
      var e = St(t);
      e = 'link[rel="' + l + '"][href="' + e + '"]', typeof a == "string" && (e += '[crossorigin="' + a + '"]'), $0.has(e) || ($0.add(e), l = { rel: l, crossOrigin: a, href: t }, u.querySelector(e) === null && (t = u.createElement("link"), Ll(t, "link", l), Hl(t), u.head.appendChild(t)));
    }
  }
  function Ry(l) {
    la.D(l), F0("dns-prefetch", l, null);
  }
  function xy(l, t) {
    la.C(l, t), F0("preconnect", l, t);
  }
  function qy(l, t, a) {
    la.L(l, t, a);
    var u = pu;
    if (u && l && t) {
      var e = 'link[rel="preload"][as="' + St(t) + '"]';
      t === "image" && a && a.imageSrcSet ? (e += '[imagesrcset="' + St(
        a.imageSrcSet
      ) + '"]', typeof a.imageSizes == "string" && (e += '[imagesizes="' + St(
        a.imageSizes
      ) + '"]')) : e += '[href="' + St(l) + '"]';
      var n = e;
      switch (t) {
        case "style":
          n = Mu(l);
          break;
        case "script":
          n = Ou(l);
      }
      pt.has(n) || (l = q(
        {
          rel: "preload",
          href: t === "image" && a && a.imageSrcSet ? void 0 : l,
          as: t
        },
        a
      ), pt.set(n, l), u.querySelector(e) !== null || t === "style" && u.querySelector(ge(n)) || t === "script" && u.querySelector(Se(n)) || (t = u.createElement("link"), Ll(t, "link", l), Hl(t), u.head.appendChild(t)));
    }
  }
  function By(l, t) {
    la.m(l, t);
    var a = pu;
    if (a && l) {
      var u = t && typeof t.as == "string" ? t.as : "script", e = 'link[rel="modulepreload"][as="' + St(u) + '"][href="' + St(l) + '"]', n = e;
      switch (u) {
        case "audioworklet":
        case "paintworklet":
        case "serviceworker":
        case "sharedworker":
        case "worker":
        case "script":
          n = Ou(l);
      }
      if (!pt.has(n) && (l = q({ rel: "modulepreload", href: l }, t), pt.set(n, l), a.querySelector(e) === null)) {
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
        u = a.createElement("link"), Ll(u, "link", l), Hl(u), a.head.appendChild(u);
      }
    }
  }
  function Cy(l, t, a) {
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
          l = q(
            { rel: "stylesheet", href: l, "data-precedence": t },
            a
          ), (a = pt.get(n)) && Pi(l, a);
          var f = c = u.createElement("link");
          Hl(f), Ll(f, "link", l), f._p = new Promise(function(y, r) {
            f.onload = y, f.onerror = r;
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
  function Yy(l, t) {
    la.X(l, t);
    var a = pu;
    if (a && l) {
      var u = ka(a).hoistableScripts, e = Ou(l), n = u.get(e);
      n || (n = a.querySelector(Se(e)), n || (l = q({ src: l, async: !0 }, t), (t = pt.get(e)) && lf(l, t), n = a.createElement("script"), Hl(n), Ll(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, u.set(e, n));
    }
  }
  function Gy(l, t) {
    la.M(l, t);
    var a = pu;
    if (a && l) {
      var u = ka(a).hoistableScripts, e = Ou(l), n = u.get(e);
      n || (n = a.querySelector(Se(e)), n || (l = q({ src: l, async: !0, type: "module" }, t), (t = pt.get(e)) && lf(l, t), n = a.createElement("script"), Hl(n), Ll(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, u.set(e, n));
    }
  }
  function I0(l, t, a, u) {
    var e = (e = V.current) ? jn(e) : null;
    if (!e) throw Error(m(446));
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
          )) && !n._p && (c.instance = n, c.state.loading = 5), pt.has(l) || (a = {
            rel: "preload",
            as: "style",
            href: a.href,
            crossOrigin: a.crossOrigin,
            integrity: a.integrity,
            media: a.media,
            hrefLang: a.hrefLang,
            referrerPolicy: a.referrerPolicy
          }, pt.set(l, a), n || Xy(
            e,
            l,
            a,
            c.state
          ))), t && u === null)
            throw Error(m(528, ""));
          return c;
        }
        if (t && u !== null)
          throw Error(m(529, ""));
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
        throw Error(m(444, l));
    }
  }
  function Mu(l) {
    return 'href="' + St(l) + '"';
  }
  function ge(l) {
    return 'link[rel="stylesheet"][' + l + "]";
  }
  function P0(l) {
    return q({}, l, {
      "data-precedence": l.precedence,
      precedence: null
    });
  }
  function Xy(l, t, a, u) {
    l.querySelector('link[rel="preload"][as="style"][' + t + "]") ? u.loading = 1 : (t = l.createElement("link"), u.preload = t, t.addEventListener("load", function() {
      return u.loading |= 1;
    }), t.addEventListener("error", function() {
      return u.loading |= 2;
    }), Ll(t, "link", a), Hl(t), l.head.appendChild(t));
  }
  function Ou(l) {
    return '[src="' + St(l) + '"]';
  }
  function Se(l) {
    return "script[async]" + l;
  }
  function ld(l, t, a) {
    if (t.count++, t.instance === null)
      switch (t.type) {
        case "style":
          var u = l.querySelector(
            'style[data-href~="' + St(a.href) + '"]'
          );
          if (u)
            return t.instance = u, Hl(u), u;
          var e = q({}, a, {
            "data-href": a.href,
            "data-precedence": a.precedence,
            href: null,
            precedence: null
          });
          return u = (l.ownerDocument || l).createElement(
            "style"
          ), Hl(u), Ll(u, "style", e), Hn(u, a.precedence, l), t.instance = u;
        case "stylesheet":
          e = Mu(a.href);
          var n = l.querySelector(
            ge(e)
          );
          if (n)
            return t.state.loading |= 4, t.instance = n, Hl(n), n;
          u = P0(a), (e = pt.get(e)) && Pi(u, e), n = (l.ownerDocument || l).createElement("link"), Hl(n);
          var c = n;
          return c._p = new Promise(function(i, f) {
            c.onload = i, c.onerror = f;
          }), Ll(n, "link", u), t.state.loading |= 4, Hn(n, a.precedence, l), t.instance = n;
        case "script":
          return n = Ou(a.src), (e = l.querySelector(
            Se(n)
          )) ? (t.instance = e, Hl(e), e) : (u = a, (e = pt.get(n)) && (u = q({}, a), lf(u, e)), l = l.ownerDocument || l, e = l.createElement("script"), Hl(e), Ll(e, "link", u), l.head.appendChild(e), t.instance = e);
        case "void":
          return null;
        default:
          throw Error(m(443, t.type));
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
  function td(l, t, a) {
    if (Rn === null) {
      var u = /* @__PURE__ */ new Map(), e = Rn = /* @__PURE__ */ new Map();
      e.set(a, u);
    } else
      e = Rn, u = e.get(a), u || (u = /* @__PURE__ */ new Map(), e.set(a, u));
    if (u.has(l)) return u;
    for (u.set(l, null), a = a.getElementsByTagName(l), e = 0; e < a.length; e++) {
      var n = a[e];
      if (!(n[Ru] || n[Gl] || l === "link" && n.getAttribute("rel") === "stylesheet") && n.namespaceURI !== "http://www.w3.org/2000/svg") {
        var c = n.getAttribute(t) || "";
        c = l + c;
        var i = u.get(c);
        i ? i.push(n) : u.set(c, [n]);
      }
    }
    return u;
  }
  function ad(l, t, a) {
    l = l.ownerDocument || l, l.head.insertBefore(
      a,
      t === "title" ? l.querySelector("head > title") : null
    );
  }
  function Qy(l, t, a) {
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
  function ud(l) {
    return !(l.type === "stylesheet" && (l.state.loading & 3) === 0);
  }
  function Zy(l, t, a, u) {
    if (a.type === "stylesheet" && (typeof u.media != "string" || matchMedia(u.media).matches !== !1) && (a.state.loading & 4) === 0) {
      if (a.instance === null) {
        var e = Mu(u.href), n = t.querySelector(
          ge(e)
        );
        if (n) {
          t = n._p, t !== null && typeof t == "object" && typeof t.then == "function" && (l.count++, l = xn.bind(l), t.then(l, l)), a.state.loading |= 4, a.instance = n, Hl(n);
          return;
        }
        n = t.ownerDocument || t, u = P0(u), (e = pt.get(e)) && Pi(u, e), n = n.createElement("link"), Hl(n);
        var c = n;
        c._p = new Promise(function(i, f) {
          c.onload = i, c.onerror = f;
        }), Ll(n, "link", u), a.instance = n;
      }
      l.stylesheets === null && (l.stylesheets = /* @__PURE__ */ new Map()), l.stylesheets.set(a, t), (t = a.state.preload) && (a.state.loading & 3) === 0 && (l.count++, a = xn.bind(l), t.addEventListener("load", a), t.addEventListener("error", a));
    }
  }
  var tf = 0;
  function Ly(l, t) {
    return l.stylesheets && l.count === 0 && Bn(l, l.stylesheets), 0 < l.count || 0 < l.imgCount ? function(a) {
      var u = setTimeout(function() {
        if (l.stylesheets && Bn(l, l.stylesheets), l.unsuspend) {
          var n = l.unsuspend;
          l.unsuspend = null, n();
        }
      }, 6e4 + t);
      0 < l.imgBytes && tf === 0 && (tf = 62500 * Ey());
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
    l.stylesheets = null, l.unsuspend !== null && (l.count++, qn = /* @__PURE__ */ new Map(), t.forEach(Vy, l), qn = null, xn.call(l));
  }
  function Vy(l, t) {
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
    $$typeof: Nl,
    Provider: null,
    Consumer: null,
    _currentValue: C,
    _currentValue2: C,
    _threadCount: 0
  };
  function Ky(l, t, a, u, e, n, c, i, f) {
    this.tag = 1, this.containerInfo = l, this.pingCache = this.current = this.pendingChildren = null, this.timeoutHandle = -1, this.callbackNode = this.next = this.pendingContext = this.context = this.cancelPendingCommit = null, this.callbackPriority = 0, this.expirationTimes = $n(-1), this.entangledLanes = this.shellSuspendCounter = this.errorRecoveryDisabledLanes = this.expiredLanes = this.warmLanes = this.pingedLanes = this.suspendedLanes = this.pendingLanes = 0, this.entanglements = $n(0), this.hiddenUpdates = $n(null), this.identifierPrefix = u, this.onUncaughtError = e, this.onCaughtError = n, this.onRecoverableError = c, this.pooledCache = null, this.pooledCacheLanes = 0, this.formState = f, this.incompleteTransitions = /* @__PURE__ */ new Map();
  }
  function ed(l, t, a, u, e, n, c, i, f, y, r, z) {
    return l = new Ky(
      l,
      t,
      a,
      c,
      f,
      y,
      r,
      z,
      i
    ), t = 1, n === !0 && (t |= 24), n = st(3, null, null, t), l.current = n, n.stateNode = l, t = xc(), t.refCount++, l.pooledCache = t, t.refCount++, n.memoizedState = {
      element: u,
      isDehydrated: a,
      cache: t
    }, Yc(n), l;
  }
  function nd(l) {
    return l ? (l = eu, l) : eu;
  }
  function cd(l, t, a, u, e, n) {
    e = nd(e), u.context === null ? u.context = e : u.pendingContext = e, u = sa(t), u.payload = { element: a }, n = n === void 0 ? null : n, n !== null && (u.callback = n), a = va(l, u, t), a !== null && (tt(a, l, t), $u(a, l, t));
  }
  function id(l, t) {
    if (l = l.memoizedState, l !== null && l.dehydrated !== null) {
      var a = l.retryLane;
      l.retryLane = a !== 0 && a < t ? a : t;
    }
  }
  function af(l, t) {
    id(l, t), (l = l.alternate) && id(l, t);
  }
  function fd(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = Ha(l, 67108864);
      t !== null && tt(t, l, 67108864), af(l, 67108864);
    }
  }
  function sd(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = mt();
      t = Fn(t);
      var a = Ha(l, t);
      a !== null && tt(a, l, t), af(l, t);
    }
  }
  var Cn = !0;
  function Jy(l, t, a, u) {
    var e = S.T;
    S.T = null;
    var n = M.p;
    try {
      M.p = 2, uf(l, t, a, u);
    } finally {
      M.p = n, S.T = e;
    }
  }
  function wy(l, t, a, u) {
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
        ), dd(l, u);
      else if (Wy(
        e,
        l,
        t,
        a,
        u
      ))
        u.stopPropagation();
      else if (dd(l, u), t & 4 && -1 < ky.indexOf(l)) {
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
                      var f = 1 << 31 - it(c);
                      i.entanglements[1] |= f, c &= ~f;
                    }
                    xt(n), (ll & 6) === 0 && (_n = nt() + 500, oe(0));
                  }
                }
                break;
              case 31:
              case 13:
                i = Ha(n, 2), i !== null && tt(i, n, 2), Tn(), af(n, 2);
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
          if (l = gl(t), l !== null) return l;
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
  function vd(l) {
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
        switch (Hd()) {
          case rf:
            return 2;
          case bf:
            return 8;
          case Me:
          case Rd:
            return 32;
          case _f:
            return 268435456;
          default:
            return 32;
        }
      default:
        return 32;
    }
  }
  var cf = !1, za = null, Ta = null, Ea = null, be = /* @__PURE__ */ new Map(), _e = /* @__PURE__ */ new Map(), Aa = [], ky = "mousedown mouseup touchcancel touchend touchstart auxclick dblclick pointercancel pointerdown pointerup dragend dragstart drop compositionend compositionstart keydown keypress keyup input textInput copy cut paste click change contextmenu reset".split(
    " "
  );
  function dd(l, t) {
    switch (l) {
      case "focusin":
      case "focusout":
        za = null;
        break;
      case "dragenter":
      case "dragleave":
        Ta = null;
        break;
      case "mouseover":
      case "mouseout":
        Ea = null;
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
    }, t !== null && (t = wa(t), t !== null && fd(t)), l) : (l.eventSystemFlags |= u, t = l.targetContainers, e !== null && t.indexOf(e) === -1 && t.push(e), l);
  }
  function Wy(l, t, a, u, e) {
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
        return Ta = ze(
          Ta,
          l,
          t,
          a,
          u,
          e
        ), !0;
      case "mouseover":
        return Ea = ze(
          Ea,
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
  function od(l) {
    var t = Ja(l.target);
    if (t !== null) {
      var a = yl(t);
      if (a !== null) {
        if (t = a.tag, t === 13) {
          if (t = ul(a), t !== null) {
            l.blockedOn = t, Mf(l.priority, function() {
              sd(a);
            });
            return;
          }
        } else if (t === 31) {
          if (t = gl(a), t !== null) {
            l.blockedOn = t, Mf(l.priority, function() {
              sd(a);
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
        return t = wa(a), t !== null && fd(t), l.blockedOn = a, !1;
      t.shift();
    }
    return !0;
  }
  function yd(l, t, a) {
    Gn(l) && a.delete(t);
  }
  function $y() {
    cf = !1, za !== null && Gn(za) && (za = null), Ta !== null && Gn(Ta) && (Ta = null), Ea !== null && Gn(Ea) && (Ea = null), be.forEach(yd), _e.forEach(yd);
  }
  function Xn(l, t) {
    l.blockedOn === t && (l.blockedOn = null, cf || (cf = !0, _.unstable_scheduleCallback(
      _.unstable_NormalPriority,
      $y
    )));
  }
  var Qn = null;
  function md(l) {
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
    za !== null && Xn(za, l), Ta !== null && Xn(Ta, l), Ea !== null && Xn(Ea, l), be.forEach(t), _e.forEach(t);
    for (var a = 0; a < Aa.length; a++) {
      var u = Aa[a];
      u.blockedOn === l && (u.blockedOn = null);
    }
    for (; 0 < Aa.length && (a = Aa[0], a.blockedOn === null); )
      od(a), a.blockedOn === null && Aa.shift();
    if (a = (l.ownerDocument || l).$$reactFormReplay, a != null)
      for (u = 0; u < a.length; u += 3) {
        var e = a[u], n = a[u + 1], c = e[Wl] || null;
        if (typeof n == "function")
          c || md(a);
        else if (c) {
          var i = null;
          if (n && n.hasAttribute("formAction")) {
            if (e = n, c = n[Wl] || null)
              i = c.formAction;
            else if (nf(e) !== null) continue;
          } else i = c.action;
          typeof i == "function" ? a[u + 1] = i : (a.splice(u, 3), u -= 3), md(a);
        }
      }
  }
  function hd() {
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
    if (t === null) throw Error(m(409));
    var a = t.current, u = mt();
    cd(a, u, l, t, null, null);
  }, Zn.prototype.unmount = ff.prototype.unmount = function() {
    var l = this._internalRoot;
    if (l !== null) {
      this._internalRoot = null;
      var t = l.containerInfo;
      cd(l.current, 2, null, l, null, null), Tn(), t[Ka] = null;
    }
  };
  function Zn(l) {
    this._internalRoot = l;
  }
  Zn.prototype.unstable_scheduleHydration = function(l) {
    if (l) {
      var t = pf();
      l = { blockedOn: null, target: l, priority: t };
      for (var a = 0; a < Aa.length && t !== 0 && t < Aa[a].priority; a++) ;
      Aa.splice(a, 0, l), a === 0 && od(l);
    }
  };
  var gd = D.version;
  if (gd !== "19.2.4")
    throw Error(
      m(
        527,
        gd,
        "19.2.4"
      )
    );
  M.findDOMNode = function(l) {
    var t = l._reactInternals;
    if (t === void 0)
      throw typeof l.render == "function" ? Error(m(188)) : (l = Object.keys(l).join(","), Error(m(268, l)));
    return l = E(t), l = l !== null ? Q(l) : null, l = l === null ? null : l.stateNode, l;
  };
  var Fy = {
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
          Fy
        ), ct = Ln;
      } catch {
      }
  }
  return Ee.createRoot = function(l, t) {
    if (!w(l)) throw Error(m(299));
    var a = !1, u = "", e = Ev, n = Av, c = pv;
    return t != null && (t.unstable_strictMode === !0 && (a = !0), t.identifierPrefix !== void 0 && (u = t.identifierPrefix), t.onUncaughtError !== void 0 && (e = t.onUncaughtError), t.onCaughtError !== void 0 && (n = t.onCaughtError), t.onRecoverableError !== void 0 && (c = t.onRecoverableError)), t = ed(
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
      hd
    ), l[Ka] = t.current, Zi(l), new ff(t);
  }, Ee.hydrateRoot = function(l, t, a) {
    if (!w(l)) throw Error(m(299));
    var u = !1, e = "", n = Ev, c = Av, i = pv, f = null;
    return a != null && (a.unstable_strictMode === !0 && (u = !0), a.identifierPrefix !== void 0 && (e = a.identifierPrefix), a.onUncaughtError !== void 0 && (n = a.onUncaughtError), a.onCaughtError !== void 0 && (c = a.onCaughtError), a.onRecoverableError !== void 0 && (i = a.onRecoverableError), a.formState !== void 0 && (f = a.formState)), t = ed(
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
      hd
    ), t.context = nd(null), a = t.current, u = mt(), u = Fn(u), e = sa(u), e.callback = null, va(a, e, u), a = u, t.current.lanes = a, Hu(t, a), xt(t), l[Ka] = t.current, Zi(l), new Zn(t);
  }, Ee.version = "19.2.4", Ee;
}
var Md;
function sm() {
  if (Md) return df.exports;
  Md = 1;
  function _() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(_);
      } catch (D) {
        console.error(D);
      }
  }
  return _(), df.exports = fm(), df.exports;
}
var vm = sm();
const dm = "_skeleton_xk662_19", om = "_card_xk662_32", ym = "_row_xk662_40", mm = "_block_xk662_47", hm = "_textStack_xk662_54", gm = "_textLine_xk662_61", Sm = "_textLineLast_xk662_68", qt = {
  skeleton: dm,
  card: om,
  row: ym,
  block: mm,
  textStack: hm,
  textLine: gm,
  textLineLast: Sm
};
function rm(_) {
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
          children: Array.from({ length: D }, (H, m) => /* @__PURE__ */ p.jsx(
            "div",
            {
              className: [
                qt.skeleton,
                qt.textLine,
                m === D - 1 && D > 1 ? qt.textLineLast : ""
              ].filter(Boolean).join(" ")
            },
            m
          ))
        }
      );
    }
  }
}
const bm = "_badge_vkl6x_1", _m = "_ready_vkl6x_12", zm = "_planning_vkl6x_18", Tm = "_implementing_vkl6x_24", Em = "_reviewing_vkl6x_30", Am = "_verifying_vkl6x_36", pm = "_done_vkl6x_42", Mm = "_cancelled_vkl6x_48", Om = "_pending_vkl6x_54", Dm = "_running_vkl6x_60", Um = "_complete_vkl6x_66", Nm = "_failed_vkl6x_72", jm = "_closed_vkl6x_78", Hm = "_blocked_vkl6x_84", Rm = "_inReview_vkl6x_90", xm = "_loading_vkl6x_96", qm = "_paused_vkl6x_102", Bm = "_unknown_vkl6x_108", xl = {
  badge: bm,
  ready: _m,
  planning: zm,
  implementing: Tm,
  reviewing: Em,
  verifying: Am,
  done: pm,
  cancelled: Mm,
  pending: Om,
  running: Dm,
  complete: Um,
  failed: Nm,
  closed: jm,
  blocked: Hm,
  inReview: Rm,
  loading: xm,
  paused: qm,
  unknown: Bm
}, Cm = {
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
}, Ym = {
  ready: xl.ready,
  planning: xl.planning,
  implementing: xl.implementing,
  reviewing: xl.reviewing,
  verifying: xl.verifying,
  done: xl.done,
  cancelled: xl.cancelled,
  pending: xl.pending,
  running: xl.running,
  complete: xl.complete,
  failed: xl.failed,
  closed: xl.closed,
  blocked: xl.blocked,
  in_review: xl.inReview,
  loading: xl.loading,
  paused: xl.paused
};
function Ae({ status: _ }) {
  const D = Cm[_] ?? _, H = Ym[_] ?? xl.unknown;
  return /* @__PURE__ */ p.jsx("span", { className: `${xl.badge} ${H}`, children: D });
}
const Gm = "_root_1ahyv_1", Xm = "_dark_1ahyv_2", Qm = "_light_1ahyv_3", Zm = "_header_1ahyv_4", Lm = "_controls_1ahyv_4", Vm = "_lifecycle_1ahyv_4", Km = "_pipRail_1ahyv_4", Jm = "_selectors_1ahyv_4", wm = "_task_1ahyv_4", km = "_event_1ahyv_4", Wm = "_connection_1ahyv_6", $m = "_meta_1ahyv_6", Fm = "_muted_1ahyv_6", Im = "_stale_1ahyv_7", Pm = "_chip_1ahyv_9", lh = "_list_1ahyv_10", th = "_card_1ahyv_11", ah = "_attention_1ahyv_11", uh = "_readiness_1ahyv_11", eh = "_waveBoard_1ahyv_13", nh = "_linkButton_1ahyv_25", ch = "_pip_1ahyv_4", ih = "_inlineMode_1ahyv_27", fh = "_fullscreen_1ahyv_27", cl = {
  root: Gm,
  dark: Xm,
  light: Qm,
  header: Zm,
  controls: Lm,
  lifecycle: Vm,
  pipRail: Km,
  selectors: Jm,
  task: wm,
  event: km,
  connection: Wm,
  meta: $m,
  muted: Fm,
  stale: Im,
  chip: Pm,
  list: lh,
  card: th,
  attention: ah,
  readiness: uh,
  waveBoard: eh,
  linkButton: nh,
  pip: ch,
  inlineMode: ih,
  fullscreen: fh
};
function sh({ agents: _ }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "agents" }),
    /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "active agents", children: _.map((D, H) => /* @__PURE__ */ p.jsxs("li", { className: cl.card, children: [
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
    ] }, `${D.task}-${D.role}-${H}`)) })
  ] });
}
function vh({ items: _, action: D }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "attention" }),
    _.length === 0 ? /* @__PURE__ */ p.jsx("p", { className: cl.muted, children: "nothing needs attention" }) : /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "attention items", children: _.map((H, m) => /* @__PURE__ */ p.jsxs("li", { className: cl.attention, children: [
      /* @__PURE__ */ p.jsxs("div", { children: [
        /* @__PURE__ */ p.jsx("strong", { children: H.kind.replace(/_/g, " ") }),
        " · ",
        H.task
      ] }),
      H.detail && /* @__PURE__ */ p.jsx("p", { children: H.detail }),
      D && /* @__PURE__ */ p.jsx("button", { onClick: () => D(`look at the blocker on ${H.task}`), children: "look at blocker" })
    ] }, `${H.task}-${H.kind}-${m}`)) })
  ] });
}
function dh({ events: _ }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "events" }),
    /* @__PURE__ */ p.jsx("ol", { className: cl.list, "aria-label": "event feed", children: _.map((D, H) => /* @__PURE__ */ p.jsxs("li", { className: cl.event, children: [
      /* @__PURE__ */ p.jsx("time", { dateTime: D.at, children: new Date(D.at).toLocaleTimeString() }),
      /* @__PURE__ */ p.jsx("span", { children: D.message })
    ] }, `${D.at}-${H}`)) })
  ] });
}
function Od({ lifecycle: _ }) {
  const D = ["planning", "ready", "implementing", "reviewing", "verifying"];
  return /* @__PURE__ */ p.jsx("div", { className: cl.lifecycle, role: "list", "aria-label": "lifecycle counts", children: D.map((H) => /* @__PURE__ */ p.jsxs("span", { role: "listitem", className: cl.chip, children: [
    /* @__PURE__ */ p.jsx("b", { children: _[H] }),
    " ",
    H
  ] }, H)) });
}
function oh() {
  return window.openai ?? {};
}
function yh() {
  const [_, D] = ql.useState(oh);
  return ql.useEffect(() => {
    const H = (m) => {
      const w = m.detail, yl = (w == null ? void 0 : w.globals) ?? w ?? {};
      D((ul) => ({ ...ul, ...yl }));
    };
    return window.addEventListener("openai:set_globals", H), () => window.removeEventListener("openai:set_globals", H);
  }, []), _;
}
function mh(_) {
  if (!_ || typeof _ != "object") return;
  const D = _;
  return D.structuredContent ?? D.structured_content ?? _;
}
function hh(_, D, H) {
  const [m, w] = ql.useState(() => _.toolOutput), [yl, ul] = ql.useState(!1), gl = ql.useRef(!1), A = ql.useRef(0), E = ql.useRef(void 0), Q = ql.useRef(!0), q = _.displayMode === "pip" || _.displayMode === "fullscreen" ? 2e3 : 3e3;
  ql.useEffect(() => {
    _.toolOutput && w(_.toolOutput);
  }, [_.toolOutput]);
  const P = ql.useCallback(async () => {
    var rl;
    if (!(gl.current || document.visibilityState !== "visible" || !((rl = window.openai) != null && rl.callTool))) {
      gl.current = !0;
      try {
        const bl = mh(await window.openai.callTool("open_monitor", { project: D, task: H }));
        bl && Q.current && w(bl), A.current = 0, Q.current && ul(!1);
      } catch {
        A.current += 1, Q.current && ul(!0);
      } finally {
        gl.current = !1;
      }
    }
  }, [D, H]);
  return ql.useEffect(() => {
    Q.current = !0;
    const rl = () => {
      window.clearInterval(E.current);
      const pl = Math.min(q * 2 ** A.current, 3e4);
      E.current = window.setInterval(async () => {
        await P(), rl();
      }, pl);
    }, bl = () => {
      document.visibilityState === "visible" ? (P(), rl()) : window.clearInterval(E.current);
    };
    return rl(), document.addEventListener("visibilitychange", bl), () => {
      Q.current = !1, window.clearInterval(E.current), document.removeEventListener("visibilitychange", bl);
    };
  }, [q, P]), { snapshot: m, stale: yl, refresh: P };
}
function gh({ focus: _, action: D }) {
  return /* @__PURE__ */ p.jsxs("section", { children: [
    /* @__PURE__ */ p.jsx("h2", { children: "waves" }),
    /* @__PURE__ */ p.jsx("div", { className: cl.waveBoard, role: "list", children: _.waves.map((H) => /* @__PURE__ */ p.jsxs("article", { role: "listitem", className: cl.card, children: [
      /* @__PURE__ */ p.jsxs("header", { children: [
        /* @__PURE__ */ p.jsxs("strong", { children: [
          "wave ",
          H.wave
        ] }),
        H.active && /* @__PURE__ */ p.jsx("span", { children: " active" })
      ] }),
      /* @__PURE__ */ p.jsx("ul", { children: H.tasks.map((m) => /* @__PURE__ */ p.jsxs("li", { children: [
        /* @__PURE__ */ p.jsxs("span", { children: [
          m.number,
          ". ",
          m.title
        ] }),
        /* @__PURE__ */ p.jsx(Ae, { status: m.status })
      ] }, m.number)) }),
      H.active && D && /* @__PURE__ */ p.jsxs("button", { onClick: () => D(`start wave ${H.wave} on ${_.filename}`), children: [
        "start wave ",
        H.wave
      ] })
    ] }, H.wave)) })
  ] });
}
function Sh() {
  var rl, bl, pl, ht, Vl, Mt, Nl, Jl, at, Bl, L, Cl, ut, Bt, et, Yl, Nt;
  const _ = yh(), D = _.displayMode ?? "inline", H = ((rl = _.widgetState) == null ? void 0 : rl.project) ?? ((bl = _.toolInput) == null ? void 0 : bl.project) ?? ((pl = _.toolOutput) == null ? void 0 : pl.project) ?? ((Vl = (ht = _.toolOutput) == null ? void 0 : ht.projects) == null ? void 0 : Vl[0]), m = ((Mt = _.widgetState) == null ? void 0 : Mt.task) ?? ((Nl = _.toolInput) == null ? void 0 : Nl.task) ?? ((at = (Jl = _.toolOutput) == null ? void 0 : Jl.focus) == null ? void 0 : at.filename) ?? ((Cl = (L = (Bl = _.toolOutput) == null ? void 0 : Bl.tasks) == null ? void 0 : L[0]) == null ? void 0 : Cl.filename), [w, yl] = ql.useState(H), [ul, gl] = ql.useState(m), { snapshot: A, stale: E } = hh(_, w, ul);
  ql.useEffect(() => {
    var N;
    !w && A && yl(A.project || ((N = A.projects) == null ? void 0 : N[0]));
  }, [w, A]), ql.useEffect(() => {
    var N, tl, S;
    !ul && A && gl(((N = A.focus) == null ? void 0 : N.filename) ?? ((S = (tl = A.tasks) == null ? void 0 : tl[0]) == null ? void 0 : S.filename));
  }, [ul, A]), ql.useEffect(() => {
    var N, tl;
    (tl = (N = window.openai) == null ? void 0 : N.setWidgetState) == null || tl.call(N, { project: w, task: ul });
  }, [w, ul]);
  const Q = _.sendFollowUpMessage ? (N) => {
    var tl, S;
    (S = (tl = window.openai) == null ? void 0 : tl.sendFollowUpMessage) == null || S.call(tl, { prompt: N });
  } : void 0, q = (A == null ? void 0 : A.attention.length) ?? 0, P = ql.useMemo(() => (A == null ? void 0 : A.active_agents.filter((N) => N.active && !N.paused).length) ?? 0, [A]);
  return A ? /* @__PURE__ */ p.jsxs("main", { className: `${cl.root} ${_.theme === "light" ? cl.light : cl.dark} ${D === "pip" ? cl.pip : D === "fullscreen" ? cl.fullscreen : cl.inlineMode}`, style: D === "inline" && _.maxHeight ? { maxHeight: _.maxHeight } : void 0, children: [
    /* @__PURE__ */ p.jsxs("header", { className: cl.header, children: [
      /* @__PURE__ */ p.jsxs("div", { children: [
        /* @__PURE__ */ p.jsx("strong", { children: "kasmos monitor" }),
        /* @__PURE__ */ p.jsx("span", { className: cl.connection, children: A.daemon_running ? "● live" : "○ daemon offline" })
      ] }),
      /* @__PURE__ */ p.jsxs("div", { className: cl.controls, children: [
        _.requestDisplayMode && D !== "pip" && /* @__PURE__ */ p.jsx("button", { "aria-label": "pin as picture in picture", onClick: () => {
          var N, tl;
          return void ((tl = (N = window.openai) == null ? void 0 : N.requestDisplayMode) == null ? void 0 : tl.call(N, { mode: "pip" }));
        }, children: "pin" }),
        _.requestDisplayMode && D !== "fullscreen" && /* @__PURE__ */ p.jsx("button", { "aria-label": "expand monitor", onClick: () => {
          var N, tl;
          return void ((tl = (N = window.openai) == null ? void 0 : N.requestDisplayMode) == null ? void 0 : tl.call(N, { mode: "fullscreen" }));
        }, children: "expand" })
      ] })
    ] }),
    /* @__PURE__ */ p.jsx("div", { className: cl.stale, "aria-live": "polite", children: E ? "stale · retrying with last known state" : "" }),
    D === "pip" ? /* @__PURE__ */ p.jsxs("div", { className: cl.pipRail, children: [
      /* @__PURE__ */ p.jsx(Od, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ p.jsxs("span", { children: [
        P,
        " running"
      ] }),
      /* @__PURE__ */ p.jsx(Ae, { status: q ? `${q} blocked` : "ready" })
    ] }) : /* @__PURE__ */ p.jsxs(p.Fragment, { children: [
      /* @__PURE__ */ p.jsx(Od, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ p.jsxs("div", { className: cl.selectors, children: [
        (((ut = A.projects) == null ? void 0 : ut.length) ?? 0) > 1 && /* @__PURE__ */ p.jsxs("label", { children: [
          "project",
          /* @__PURE__ */ p.jsx("select", { value: w, onChange: (N) => {
            yl(N.target.value), gl(void 0);
          }, children: (Bt = A.projects) == null ? void 0 : Bt.map((N) => /* @__PURE__ */ p.jsx("option", { children: N }, N)) })
        ] }),
        (((et = A.tasks) == null ? void 0 : et.length) ?? 0) > 0 && /* @__PURE__ */ p.jsxs("label", { children: [
          "task",
          /* @__PURE__ */ p.jsx("select", { value: ul, onChange: (N) => gl(N.target.value), children: (Yl = A.tasks) == null ? void 0 : Yl.map((N) => /* @__PURE__ */ p.jsx("option", { value: N.filename, children: N.filename }, N.filename)) })
        ] })
      ] }),
      /* @__PURE__ */ p.jsxs("section", { children: [
        /* @__PURE__ */ p.jsx("h2", { children: "tasks" }),
        /* @__PURE__ */ p.jsx("ul", { className: cl.list, "aria-label": "tasks", children: (Nt = A.tasks) == null ? void 0 : Nt.map((N) => {
          const tl = N.subtasks_total ? Math.round(N.subtasks_done / N.subtasks_total * 100) : 0;
          return /* @__PURE__ */ p.jsxs("li", { className: cl.task, children: [
            /* @__PURE__ */ p.jsxs("div", { children: [
              /* @__PURE__ */ p.jsx("button", { className: cl.linkButton, onClick: () => gl(N.filename), children: N.filename }),
              /* @__PURE__ */ p.jsx(Ae, { status: N.blocked ? "blocked" : N.status })
            ] }),
            /* @__PURE__ */ p.jsx("progress", { value: N.subtasks_done, max: N.subtasks_total || 1, "aria-label": `${N.filename} progress` }),
            /* @__PURE__ */ p.jsxs("small", { children: [
              tl,
              "% · wave ",
              N.active_wave || 0,
              "/",
              N.total_waves || 0
            ] })
          ] }, N.filename);
        }) })
      ] }),
      /* @__PURE__ */ p.jsx(vh, { items: A.attention, action: Q }),
      D === "fullscreen" && /* @__PURE__ */ p.jsxs(p.Fragment, { children: [
        A.focus && /* @__PURE__ */ p.jsx(gh, { focus: A.focus, action: Q }),
        /* @__PURE__ */ p.jsx(sh, { agents: A.active_agents }),
        A.focus && /* @__PURE__ */ p.jsxs("section", { className: cl.readiness, children: [
          /* @__PURE__ */ p.jsx("h2", { children: "readiness" }),
          /* @__PURE__ */ p.jsx(Ae, { status: A.focus.readiness.status }),
          /* @__PURE__ */ p.jsx("p", { children: A.focus.readiness.last_verify_outcome || "no verification outcome" }),
          A.focus.readiness.has_review_feedback && Q && /* @__PURE__ */ p.jsx("button", { onClick: () => {
            var N;
            return Q(`approve review for ${(N = A.focus) == null ? void 0 : N.filename}`);
          }, children: "approve review" })
        ] }),
        /* @__PURE__ */ p.jsx(dh, { events: A.events ?? [] })
      ] })
    ] })
  ] }) : /* @__PURE__ */ p.jsx("main", { className: cl.root, children: /* @__PURE__ */ p.jsx(rm, { variant: "text", lines: 4 }) });
}
const Dd = document.getElementById("root");
Dd && vm.createRoot(Dd).render(/* @__PURE__ */ p.jsx(um.StrictMode, { children: /* @__PURE__ */ p.jsx(Sh, {}) }));
