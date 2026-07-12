function am(v) {
  return v && v.__esModule && Object.prototype.hasOwnProperty.call(v, "default") ? v.default : v;
}
var of = { exports: {} }, Eu = {};
/**
 * @license React
 * react-jsx-runtime.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var bv;
function em() {
  if (bv) return Eu;
  bv = 1;
  var v = Symbol.for("react.transitional.element"), O = Symbol.for("react.fragment");
  function D(s, x, W) {
    var P = null;
    if (W !== void 0 && (P = "" + W), x.key !== void 0 && (P = "" + x.key), "key" in x) {
      W = {};
      for (var gl in x)
        gl !== "key" && (W[gl] = x[gl]);
    } else W = x;
    return x = W.ref, {
      $$typeof: v,
      type: s,
      key: P,
      ref: x !== void 0 ? x : null,
      props: W
    };
  }
  return Eu.Fragment = O, Eu.jsx = D, Eu.jsxs = D, Eu;
}
var _v;
function um() {
  return _v || (_v = 1, of.exports = em()), of.exports;
}
var M = um(), df = { exports: {} }, G = {};
/**
 * @license React
 * react.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var zv;
function nm() {
  if (zv) return G;
  zv = 1;
  var v = Symbol.for("react.transitional.element"), O = Symbol.for("react.portal"), D = Symbol.for("react.fragment"), s = Symbol.for("react.strict_mode"), x = Symbol.for("react.profiler"), W = Symbol.for("react.consumer"), P = Symbol.for("react.context"), gl = Symbol.for("react.forward_ref"), A = Symbol.for("react.suspense"), p = Symbol.for("react.memo"), L = Symbol.for("react.lazy"), q = Symbol.for("react.activity"), el = Symbol.iterator;
  function Sl(d) {
    return d === null || typeof d != "object" ? null : (d = el && d[el] || d["@@iterator"], typeof d == "function" ? d : null);
  }
  var zl = {
    isMounted: function() {
      return !1;
    },
    enqueueForceUpdate: function() {
    },
    enqueueReplaceState: function() {
    },
    enqueueSetState: function() {
    }
  }, Ml = Object.assign, Wl = {};
  function Ol(d, T, U) {
    this.props = d, this.context = T, this.refs = Wl, this.updater = U || zl;
  }
  Ol.prototype.isReactComponent = {}, Ol.prototype.setState = function(d, T) {
    if (typeof d != "object" && typeof d != "function" && d != null)
      throw Error(
        "takes an object of state variables to update or a function which returns an object of state variables."
      );
    this.updater.enqueueSetState(this, d, T, "setState");
  }, Ol.prototype.forceUpdate = function(d) {
    this.updater.enqueueForceUpdate(this, d, "forceUpdate");
  };
  function $l() {
  }
  $l.prototype = Ol.prototype;
  function rl(d, T, U) {
    this.props = d, this.context = T, this.refs = Wl, this.updater = U || zl;
  }
  var El = rl.prototype = new $l();
  El.constructor = rl, Ml(El, Ol.prototype), El.isPureReactComponent = !0;
  var Jl = Array.isArray;
  function Dl() {
  }
  var Y = { H: null, A: null, T: null, S: null }, Cl = Object.prototype.hasOwnProperty;
  function Rl(d, T, U) {
    var j = U.ref;
    return {
      $$typeof: v,
      type: d,
      key: T,
      ref: j !== void 0 ? j : null,
      props: U
    };
  }
  function wl(d, T) {
    return Rl(d.type, T, d.props);
  }
  function xl(d) {
    return typeof d == "object" && d !== null && d.$$typeof === v;
  }
  function ql(d) {
    var T = { "=": "=0", ":": "=2" };
    return "$" + d.replace(/[=:]/g, function(U) {
      return T[U];
    });
  }
  var gt = /\/+/g;
  function St(d, T) {
    return typeof d == "object" && d !== null && d.key != null ? ql("" + d.key) : T.toString(36);
  }
  function Pl(d) {
    switch (d.status) {
      case "fulfilled":
        return d.value;
      case "rejected":
        throw d.reason;
      default:
        switch (typeof d.status == "string" ? d.then(Dl, Dl) : (d.status = "pending", d.then(
          function(T) {
            d.status === "pending" && (d.status = "fulfilled", d.value = T);
          },
          function(T) {
            d.status === "pending" && (d.status = "rejected", d.reason = T);
          }
        )), d.status) {
          case "fulfilled":
            return d.value;
          case "rejected":
            throw d.reason;
        }
    }
    throw d;
  }
  function b(d, T, U, j, X) {
    var V = typeof d;
    (V === "undefined" || V === "boolean") && (d = null);
    var il = !1;
    if (d === null) il = !0;
    else
      switch (V) {
        case "bigint":
        case "string":
        case "number":
          il = !0;
          break;
        case "object":
          switch (d.$$typeof) {
            case v:
            case O:
              il = !0;
              break;
            case L:
              return il = d._init, b(
                il(d._payload),
                T,
                U,
                j,
                X
              );
          }
      }
    if (il)
      return X = X(d), il = j === "" ? "." + St(d, 0) : j, Jl(X) ? (U = "", il != null && (U = il.replace(gt, "$&/") + "/"), b(X, T, U, "", function(Ue) {
        return Ue;
      })) : X != null && (xl(X) && (X = wl(
        X,
        U + (X.key == null || d && d.key === X.key ? "" : ("" + X.key).replace(
          gt,
          "$&/"
        ) + "/") + il
      )), T.push(X)), 1;
    il = 0;
    var Fl = j === "" ? "." : j + ":";
    if (Jl(d))
      for (var Tl = 0; Tl < d.length; Tl++)
        j = d[Tl], V = Fl + St(j, Tl), il += b(
          j,
          T,
          U,
          V,
          X
        );
    else if (Tl = Sl(d), typeof Tl == "function")
      for (d = Tl.call(d), Tl = 0; !(j = d.next()).done; )
        j = j.value, V = Fl + St(j, Tl++), il += b(
          j,
          T,
          U,
          V,
          X
        );
    else if (V === "object") {
      if (typeof d.then == "function")
        return b(
          Pl(d),
          T,
          U,
          j,
          X
        );
      throw T = String(d), Error(
        "Objects are not valid as a React child (found: " + (T === "[object Object]" ? "object with keys {" + Object.keys(d).join(", ") + "}" : T) + "). If you meant to render a collection of children, use an array instead."
      );
    }
    return il;
  }
  function _(d, T, U) {
    if (d == null) return d;
    var j = [], X = 0;
    return b(d, j, "", "", function(V) {
      return T.call(U, V, X++);
    }), j;
  }
  function R(d) {
    if (d._status === -1) {
      var T = d._result;
      T = T(), T.then(
        function(U) {
          (d._status === 0 || d._status === -1) && (d._status = 1, d._result = U);
        },
        function(U) {
          (d._status === 0 || d._status === -1) && (d._status = 2, d._result = U);
        }
      ), d._status === -1 && (d._status = 0, d._result = T);
    }
    if (d._status === 1) return d._result.default;
    throw d._result;
  }
  var F = typeof reportError == "function" ? reportError : function(d) {
    if (typeof window == "object" && typeof window.ErrorEvent == "function") {
      var T = new window.ErrorEvent("error", {
        bubbles: !0,
        cancelable: !0,
        message: typeof d == "object" && d !== null && typeof d.message == "string" ? String(d.message) : String(d),
        error: d
      });
      if (!window.dispatchEvent(T)) return;
    } else if (typeof process == "object" && typeof process.emit == "function") {
      process.emit("uncaughtException", d);
      return;
    }
    console.error(d);
  }, nl = {
    map: _,
    forEach: function(d, T, U) {
      _(
        d,
        function() {
          T.apply(this, arguments);
        },
        U
      );
    },
    count: function(d) {
      var T = 0;
      return _(d, function() {
        T++;
      }), T;
    },
    toArray: function(d) {
      return _(d, function(T) {
        return T;
      }) || [];
    },
    only: function(d) {
      if (!xl(d))
        throw Error(
          "React.Children.only expected to receive a single React element child."
        );
      return d;
    }
  };
  return G.Activity = q, G.Children = nl, G.Component = Ol, G.Fragment = D, G.Profiler = x, G.PureComponent = rl, G.StrictMode = s, G.Suspense = A, G.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = Y, G.__COMPILER_RUNTIME = {
    __proto__: null,
    c: function(d) {
      return Y.H.useMemoCache(d);
    }
  }, G.cache = function(d) {
    return function() {
      return d.apply(null, arguments);
    };
  }, G.cacheSignal = function() {
    return null;
  }, G.cloneElement = function(d, T, U) {
    if (d == null)
      throw Error(
        "The argument must be a React element, but you passed " + d + "."
      );
    var j = Ml({}, d.props), X = d.key;
    if (T != null)
      for (V in T.key !== void 0 && (X = "" + T.key), T)
        !Cl.call(T, V) || V === "key" || V === "__self" || V === "__source" || V === "ref" && T.ref === void 0 || (j[V] = T[V]);
    var V = arguments.length - 2;
    if (V === 1) j.children = U;
    else if (1 < V) {
      for (var il = Array(V), Fl = 0; Fl < V; Fl++)
        il[Fl] = arguments[Fl + 2];
      j.children = il;
    }
    return Rl(d.type, X, j);
  }, G.createContext = function(d) {
    return d = {
      $$typeof: P,
      _currentValue: d,
      _currentValue2: d,
      _threadCount: 0,
      Provider: null,
      Consumer: null
    }, d.Provider = d, d.Consumer = {
      $$typeof: W,
      _context: d
    }, d;
  }, G.createElement = function(d, T, U) {
    var j, X = {}, V = null;
    if (T != null)
      for (j in T.key !== void 0 && (V = "" + T.key), T)
        Cl.call(T, j) && j !== "key" && j !== "__self" && j !== "__source" && (X[j] = T[j]);
    var il = arguments.length - 2;
    if (il === 1) X.children = U;
    else if (1 < il) {
      for (var Fl = Array(il), Tl = 0; Tl < il; Tl++)
        Fl[Tl] = arguments[Tl + 2];
      X.children = Fl;
    }
    if (d && d.defaultProps)
      for (j in il = d.defaultProps, il)
        X[j] === void 0 && (X[j] = il[j]);
    return Rl(d, V, X);
  }, G.createRef = function() {
    return { current: null };
  }, G.forwardRef = function(d) {
    return { $$typeof: gl, render: d };
  }, G.isValidElement = xl, G.lazy = function(d) {
    return {
      $$typeof: L,
      _payload: { _status: -1, _result: d },
      _init: R
    };
  }, G.memo = function(d, T) {
    return {
      $$typeof: p,
      type: d,
      compare: T === void 0 ? null : T
    };
  }, G.startTransition = function(d) {
    var T = Y.T, U = {};
    Y.T = U;
    try {
      var j = d(), X = Y.S;
      X !== null && X(U, j), typeof j == "object" && j !== null && typeof j.then == "function" && j.then(Dl, F);
    } catch (V) {
      F(V);
    } finally {
      T !== null && U.types !== null && (T.types = U.types), Y.T = T;
    }
  }, G.unstable_useCacheRefresh = function() {
    return Y.H.useCacheRefresh();
  }, G.use = function(d) {
    return Y.H.use(d);
  }, G.useActionState = function(d, T, U) {
    return Y.H.useActionState(d, T, U);
  }, G.useCallback = function(d, T) {
    return Y.H.useCallback(d, T);
  }, G.useContext = function(d) {
    return Y.H.useContext(d);
  }, G.useDebugValue = function() {
  }, G.useDeferredValue = function(d, T) {
    return Y.H.useDeferredValue(d, T);
  }, G.useEffect = function(d, T) {
    return Y.H.useEffect(d, T);
  }, G.useEffectEvent = function(d) {
    return Y.H.useEffectEvent(d);
  }, G.useId = function() {
    return Y.H.useId();
  }, G.useImperativeHandle = function(d, T, U) {
    return Y.H.useImperativeHandle(d, T, U);
  }, G.useInsertionEffect = function(d, T) {
    return Y.H.useInsertionEffect(d, T);
  }, G.useLayoutEffect = function(d, T) {
    return Y.H.useLayoutEffect(d, T);
  }, G.useMemo = function(d, T) {
    return Y.H.useMemo(d, T);
  }, G.useOptimistic = function(d, T) {
    return Y.H.useOptimistic(d, T);
  }, G.useReducer = function(d, T, U) {
    return Y.H.useReducer(d, T, U);
  }, G.useRef = function(d) {
    return Y.H.useRef(d);
  }, G.useState = function(d) {
    return Y.H.useState(d);
  }, G.useSyncExternalStore = function(d, T, U) {
    return Y.H.useSyncExternalStore(
      d,
      T,
      U
    );
  }, G.useTransition = function() {
    return Y.H.useTransition();
  }, G.version = "19.2.4", G;
}
var Ev;
function rf() {
  return Ev || (Ev = 1, df.exports = nm()), df.exports;
}
var I = rf();
const im = /* @__PURE__ */ am(I);
var vf = { exports: {} }, Tu = {}, yf = { exports: {} }, mf = {};
/**
 * @license React
 * scheduler.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Tv;
function cm() {
  return Tv || (Tv = 1, (function(v) {
    function O(b, _) {
      var R = b.length;
      b.push(_);
      l: for (; 0 < R; ) {
        var F = R - 1 >>> 1, nl = b[F];
        if (0 < x(nl, _))
          b[F] = _, b[R] = nl, R = F;
        else break l;
      }
    }
    function D(b) {
      return b.length === 0 ? null : b[0];
    }
    function s(b) {
      if (b.length === 0) return null;
      var _ = b[0], R = b.pop();
      if (R !== _) {
        b[0] = R;
        l: for (var F = 0, nl = b.length, d = nl >>> 1; F < d; ) {
          var T = 2 * (F + 1) - 1, U = b[T], j = T + 1, X = b[j];
          if (0 > x(U, R))
            j < nl && 0 > x(X, U) ? (b[F] = X, b[j] = R, F = j) : (b[F] = U, b[T] = R, F = T);
          else if (j < nl && 0 > x(X, R))
            b[F] = X, b[j] = R, F = j;
          else break l;
        }
      }
      return _;
    }
    function x(b, _) {
      var R = b.sortIndex - _.sortIndex;
      return R !== 0 ? R : b.id - _.id;
    }
    if (v.unstable_now = void 0, typeof performance == "object" && typeof performance.now == "function") {
      var W = performance;
      v.unstable_now = function() {
        return W.now();
      };
    } else {
      var P = Date, gl = P.now();
      v.unstable_now = function() {
        return P.now() - gl;
      };
    }
    var A = [], p = [], L = 1, q = null, el = 3, Sl = !1, zl = !1, Ml = !1, Wl = !1, Ol = typeof setTimeout == "function" ? setTimeout : null, $l = typeof clearTimeout == "function" ? clearTimeout : null, rl = typeof setImmediate < "u" ? setImmediate : null;
    function El(b) {
      for (var _ = D(p); _ !== null; ) {
        if (_.callback === null) s(p);
        else if (_.startTime <= b)
          s(p), _.sortIndex = _.expirationTime, O(A, _);
        else break;
        _ = D(p);
      }
    }
    function Jl(b) {
      if (Ml = !1, El(b), !zl)
        if (D(A) !== null)
          zl = !0, Dl || (Dl = !0, ql());
        else {
          var _ = D(p);
          _ !== null && Pl(Jl, _.startTime - b);
        }
    }
    var Dl = !1, Y = -1, Cl = 5, Rl = -1;
    function wl() {
      return Wl ? !0 : !(v.unstable_now() - Rl < Cl);
    }
    function xl() {
      if (Wl = !1, Dl) {
        var b = v.unstable_now();
        Rl = b;
        var _ = !0;
        try {
          l: {
            zl = !1, Ml && (Ml = !1, $l(Y), Y = -1), Sl = !0;
            var R = el;
            try {
              t: {
                for (El(b), q = D(A); q !== null && !(q.expirationTime > b && wl()); ) {
                  var F = q.callback;
                  if (typeof F == "function") {
                    q.callback = null, el = q.priorityLevel;
                    var nl = F(
                      q.expirationTime <= b
                    );
                    if (b = v.unstable_now(), typeof nl == "function") {
                      q.callback = nl, El(b), _ = !0;
                      break t;
                    }
                    q === D(A) && s(A), El(b);
                  } else s(A);
                  q = D(A);
                }
                if (q !== null) _ = !0;
                else {
                  var d = D(p);
                  d !== null && Pl(
                    Jl,
                    d.startTime - b
                  ), _ = !1;
                }
              }
              break l;
            } finally {
              q = null, el = R, Sl = !1;
            }
            _ = void 0;
          }
        } finally {
          _ ? ql() : Dl = !1;
        }
      }
    }
    var ql;
    if (typeof rl == "function")
      ql = function() {
        rl(xl);
      };
    else if (typeof MessageChannel < "u") {
      var gt = new MessageChannel(), St = gt.port2;
      gt.port1.onmessage = xl, ql = function() {
        St.postMessage(null);
      };
    } else
      ql = function() {
        Ol(xl, 0);
      };
    function Pl(b, _) {
      Y = Ol(function() {
        b(v.unstable_now());
      }, _);
    }
    v.unstable_IdlePriority = 5, v.unstable_ImmediatePriority = 1, v.unstable_LowPriority = 4, v.unstable_NormalPriority = 3, v.unstable_Profiling = null, v.unstable_UserBlockingPriority = 2, v.unstable_cancelCallback = function(b) {
      b.callback = null;
    }, v.unstable_forceFrameRate = function(b) {
      0 > b || 125 < b ? console.error(
        "forceFrameRate takes a positive int between 0 and 125, forcing frame rates higher than 125 fps is not supported"
      ) : Cl = 0 < b ? Math.floor(1e3 / b) : 5;
    }, v.unstable_getCurrentPriorityLevel = function() {
      return el;
    }, v.unstable_next = function(b) {
      switch (el) {
        case 1:
        case 2:
        case 3:
          var _ = 3;
          break;
        default:
          _ = el;
      }
      var R = el;
      el = _;
      try {
        return b();
      } finally {
        el = R;
      }
    }, v.unstable_requestPaint = function() {
      Wl = !0;
    }, v.unstable_runWithPriority = function(b, _) {
      switch (b) {
        case 1:
        case 2:
        case 3:
        case 4:
        case 5:
          break;
        default:
          b = 3;
      }
      var R = el;
      el = b;
      try {
        return _();
      } finally {
        el = R;
      }
    }, v.unstable_scheduleCallback = function(b, _, R) {
      var F = v.unstable_now();
      switch (typeof R == "object" && R !== null ? (R = R.delay, R = typeof R == "number" && 0 < R ? F + R : F) : R = F, b) {
        case 1:
          var nl = -1;
          break;
        case 2:
          nl = 250;
          break;
        case 5:
          nl = 1073741823;
          break;
        case 4:
          nl = 1e4;
          break;
        default:
          nl = 5e3;
      }
      return nl = R + nl, b = {
        id: L++,
        callback: _,
        priorityLevel: b,
        startTime: R,
        expirationTime: nl,
        sortIndex: -1
      }, R > F ? (b.sortIndex = R, O(p, b), D(A) === null && b === D(p) && (Ml ? ($l(Y), Y = -1) : Ml = !0, Pl(Jl, R - F))) : (b.sortIndex = nl, O(A, b), zl || Sl || (zl = !0, Dl || (Dl = !0, ql()))), b;
    }, v.unstable_shouldYield = wl, v.unstable_wrapCallback = function(b) {
      var _ = el;
      return function() {
        var R = el;
        el = _;
        try {
          return b.apply(this, arguments);
        } finally {
          el = R;
        }
      };
    };
  })(mf)), mf;
}
var Av;
function fm() {
  return Av || (Av = 1, yf.exports = cm()), yf.exports;
}
var hf = { exports: {} }, kl = {};
/**
 * @license React
 * react-dom.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var pv;
function sm() {
  if (pv) return kl;
  pv = 1;
  var v = rf();
  function O(A) {
    var p = "https://react.dev/errors/" + A;
    if (1 < arguments.length) {
      p += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var L = 2; L < arguments.length; L++)
        p += "&args[]=" + encodeURIComponent(arguments[L]);
    }
    return "Minified React error #" + A + "; visit " + p + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function D() {
  }
  var s = {
    d: {
      f: D,
      r: function() {
        throw Error(O(522));
      },
      D,
      C: D,
      L: D,
      m: D,
      X: D,
      S: D,
      M: D
    },
    p: 0,
    findDOMNode: null
  }, x = Symbol.for("react.portal");
  function W(A, p, L) {
    var q = 3 < arguments.length && arguments[3] !== void 0 ? arguments[3] : null;
    return {
      $$typeof: x,
      key: q == null ? null : "" + q,
      children: A,
      containerInfo: p,
      implementation: L
    };
  }
  var P = v.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE;
  function gl(A, p) {
    if (A === "font") return "";
    if (typeof p == "string")
      return p === "use-credentials" ? p : "";
  }
  return kl.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE = s, kl.createPortal = function(A, p) {
    var L = 2 < arguments.length && arguments[2] !== void 0 ? arguments[2] : null;
    if (!p || p.nodeType !== 1 && p.nodeType !== 9 && p.nodeType !== 11)
      throw Error(O(299));
    return W(A, p, null, L);
  }, kl.flushSync = function(A) {
    var p = P.T, L = s.p;
    try {
      if (P.T = null, s.p = 2, A) return A();
    } finally {
      P.T = p, s.p = L, s.d.f();
    }
  }, kl.preconnect = function(A, p) {
    typeof A == "string" && (p ? (p = p.crossOrigin, p = typeof p == "string" ? p === "use-credentials" ? p : "" : void 0) : p = null, s.d.C(A, p));
  }, kl.prefetchDNS = function(A) {
    typeof A == "string" && s.d.D(A);
  }, kl.preinit = function(A, p) {
    if (typeof A == "string" && p && typeof p.as == "string") {
      var L = p.as, q = gl(L, p.crossOrigin), el = typeof p.integrity == "string" ? p.integrity : void 0, Sl = typeof p.fetchPriority == "string" ? p.fetchPriority : void 0;
      L === "style" ? s.d.S(
        A,
        typeof p.precedence == "string" ? p.precedence : void 0,
        {
          crossOrigin: q,
          integrity: el,
          fetchPriority: Sl
        }
      ) : L === "script" && s.d.X(A, {
        crossOrigin: q,
        integrity: el,
        fetchPriority: Sl,
        nonce: typeof p.nonce == "string" ? p.nonce : void 0
      });
    }
  }, kl.preinitModule = function(A, p) {
    if (typeof A == "string")
      if (typeof p == "object" && p !== null) {
        if (p.as == null || p.as === "script") {
          var L = gl(
            p.as,
            p.crossOrigin
          );
          s.d.M(A, {
            crossOrigin: L,
            integrity: typeof p.integrity == "string" ? p.integrity : void 0,
            nonce: typeof p.nonce == "string" ? p.nonce : void 0
          });
        }
      } else p == null && s.d.M(A);
  }, kl.preload = function(A, p) {
    if (typeof A == "string" && typeof p == "object" && p !== null && typeof p.as == "string") {
      var L = p.as, q = gl(L, p.crossOrigin);
      s.d.L(A, L, {
        crossOrigin: q,
        integrity: typeof p.integrity == "string" ? p.integrity : void 0,
        nonce: typeof p.nonce == "string" ? p.nonce : void 0,
        type: typeof p.type == "string" ? p.type : void 0,
        fetchPriority: typeof p.fetchPriority == "string" ? p.fetchPriority : void 0,
        referrerPolicy: typeof p.referrerPolicy == "string" ? p.referrerPolicy : void 0,
        imageSrcSet: typeof p.imageSrcSet == "string" ? p.imageSrcSet : void 0,
        imageSizes: typeof p.imageSizes == "string" ? p.imageSizes : void 0,
        media: typeof p.media == "string" ? p.media : void 0
      });
    }
  }, kl.preloadModule = function(A, p) {
    if (typeof A == "string")
      if (p) {
        var L = gl(p.as, p.crossOrigin);
        s.d.m(A, {
          as: typeof p.as == "string" && p.as !== "script" ? p.as : void 0,
          crossOrigin: L,
          integrity: typeof p.integrity == "string" ? p.integrity : void 0
        });
      } else s.d.m(A);
  }, kl.requestFormReset = function(A) {
    s.d.r(A);
  }, kl.unstable_batchedUpdates = function(A, p) {
    return A(p);
  }, kl.useFormState = function(A, p, L) {
    return P.H.useFormState(A, p, L);
  }, kl.useFormStatus = function() {
    return P.H.useHostTransitionStatus();
  }, kl.version = "19.2.4", kl;
}
var Mv;
function om() {
  if (Mv) return hf.exports;
  Mv = 1;
  function v() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(v);
      } catch (O) {
        console.error(O);
      }
  }
  return v(), hf.exports = sm(), hf.exports;
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
var Ov;
function dm() {
  if (Ov) return Tu;
  Ov = 1;
  var v = fm(), O = rf(), D = om();
  function s(l) {
    var t = "https://react.dev/errors/" + l;
    if (1 < arguments.length) {
      t += "?args[]=" + encodeURIComponent(arguments[1]);
      for (var a = 2; a < arguments.length; a++)
        t += "&args[]=" + encodeURIComponent(arguments[a]);
    }
    return "Minified React error #" + l + "; visit " + t + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings.";
  }
  function x(l) {
    return !(!l || l.nodeType !== 1 && l.nodeType !== 9 && l.nodeType !== 11);
  }
  function W(l) {
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
  function P(l) {
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
    if (W(l) !== l)
      throw Error(s(188));
  }
  function p(l) {
    var t = l.alternate;
    if (!t) {
      if (t = W(l), t === null) throw Error(s(188));
      return t !== l ? null : l;
    }
    for (var a = l, e = t; ; ) {
      var u = a.return;
      if (u === null) break;
      var n = u.alternate;
      if (n === null) {
        if (e = u.return, e !== null) {
          a = e;
          continue;
        }
        break;
      }
      if (u.child === n.child) {
        for (n = u.child; n; ) {
          if (n === a) return A(u), l;
          if (n === e) return A(u), t;
          n = n.sibling;
        }
        throw Error(s(188));
      }
      if (a.return !== e.return) a = u, e = n;
      else {
        for (var i = !1, c = u.child; c; ) {
          if (c === a) {
            i = !0, a = u, e = n;
            break;
          }
          if (c === e) {
            i = !0, e = u, a = n;
            break;
          }
          c = c.sibling;
        }
        if (!i) {
          for (c = n.child; c; ) {
            if (c === a) {
              i = !0, a = n, e = u;
              break;
            }
            if (c === e) {
              i = !0, e = n, a = u;
              break;
            }
            c = c.sibling;
          }
          if (!i) throw Error(s(189));
        }
      }
      if (a.alternate !== e) throw Error(s(190));
    }
    if (a.tag !== 3) throw Error(s(188));
    return a.stateNode.current === a ? l : t;
  }
  function L(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l;
    for (l = l.child; l !== null; ) {
      if (t = L(l), t !== null) return t;
      l = l.sibling;
    }
    return null;
  }
  var q = Object.assign, el = Symbol.for("react.element"), Sl = Symbol.for("react.transitional.element"), zl = Symbol.for("react.portal"), Ml = Symbol.for("react.fragment"), Wl = Symbol.for("react.strict_mode"), Ol = Symbol.for("react.profiler"), $l = Symbol.for("react.consumer"), rl = Symbol.for("react.context"), El = Symbol.for("react.forward_ref"), Jl = Symbol.for("react.suspense"), Dl = Symbol.for("react.suspense_list"), Y = Symbol.for("react.memo"), Cl = Symbol.for("react.lazy"), Rl = Symbol.for("react.activity"), wl = Symbol.for("react.memo_cache_sentinel"), xl = Symbol.iterator;
  function ql(l) {
    return l === null || typeof l != "object" ? null : (l = xl && l[xl] || l["@@iterator"], typeof l == "function" ? l : null);
  }
  var gt = Symbol.for("react.client.reference");
  function St(l) {
    if (l == null) return null;
    if (typeof l == "function")
      return l.$$typeof === gt ? null : l.displayName || l.name || null;
    if (typeof l == "string") return l;
    switch (l) {
      case Ml:
        return "Fragment";
      case Ol:
        return "Profiler";
      case Wl:
        return "StrictMode";
      case Jl:
        return "Suspense";
      case Dl:
        return "SuspenseList";
      case Rl:
        return "Activity";
    }
    if (typeof l == "object")
      switch (l.$$typeof) {
        case zl:
          return "Portal";
        case rl:
          return l.displayName || "Context";
        case $l:
          return (l._context.displayName || "Context") + ".Consumer";
        case El:
          var t = l.render;
          return l = l.displayName, l || (l = t.displayName || t.name || "", l = l !== "" ? "ForwardRef(" + l + ")" : "ForwardRef"), l;
        case Y:
          return t = l.displayName || null, t !== null ? t : St(l.type) || "Memo";
        case Cl:
          t = l._payload, l = l._init;
          try {
            return St(l(t));
          } catch {
          }
      }
    return null;
  }
  var Pl = Array.isArray, b = O.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, _ = D.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE, R = {
    pending: !1,
    data: null,
    method: null,
    action: null
  }, F = [], nl = -1;
  function d(l) {
    return { current: l };
  }
  function T(l) {
    0 > nl || (l.current = F[nl], F[nl] = null, nl--);
  }
  function U(l, t) {
    nl++, F[nl] = l.current, l.current = t;
  }
  var j = d(null), X = d(null), V = d(null), il = d(null);
  function Fl(l, t) {
    switch (U(V, t), U(X, l), U(j, null), t.nodeType) {
      case 9:
      case 11:
        l = (l = t.documentElement) && (l = l.namespaceURI) ? Zd(l) : 0;
        break;
      default:
        if (l = t.tagName, t = t.namespaceURI)
          t = Zd(t), l = Ld(t, l);
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
    T(j), U(j, l);
  }
  function Tl() {
    T(j), T(X), T(V);
  }
  function Ue(l) {
    l.memoizedState !== null && U(il, l);
    var t = j.current, a = Ld(t, l.type);
    t !== a && (U(X, l), U(j, a));
  }
  function pu(l) {
    X.current === l && (T(j), T(X)), il.current === l && (T(il), Su._currentValue = R);
  }
  var Kn, gf;
  function Ma(l) {
    if (Kn === void 0)
      try {
        throw Error();
      } catch (a) {
        var t = a.stack.trim().match(/\n( *(at )?)/);
        Kn = t && t[1] || "", gf = -1 < a.stack.indexOf(`
    at`) ? " (<anonymous>)" : -1 < a.stack.indexOf("@") ? "@unknown:0:0" : "";
      }
    return `
` + Kn + l + gf;
  }
  var Jn = !1;
  function wn(l, t) {
    if (!l || Jn) return "";
    Jn = !0;
    var a = Error.prepareStackTrace;
    Error.prepareStackTrace = void 0;
    try {
      var e = {
        DetermineComponentFrameRoot: function() {
          try {
            if (t) {
              var E = function() {
                throw Error();
              };
              if (Object.defineProperty(E.prototype, "props", {
                set: function() {
                  throw Error();
                }
              }), typeof Reflect == "object" && Reflect.construct) {
                try {
                  Reflect.construct(E, []);
                } catch (g) {
                  var r = g;
                }
                Reflect.construct(l, [], E);
              } else {
                try {
                  E.call();
                } catch (g) {
                  r = g;
                }
                l.call(E.prototype);
              }
            } else {
              try {
                throw Error();
              } catch (g) {
                r = g;
              }
              (E = l()) && typeof E.catch == "function" && E.catch(function() {
              });
            }
          } catch (g) {
            if (g && r && typeof g.stack == "string")
              return [g.stack, r.stack];
          }
          return [null, null];
        }
      };
      e.DetermineComponentFrameRoot.displayName = "DetermineComponentFrameRoot";
      var u = Object.getOwnPropertyDescriptor(
        e.DetermineComponentFrameRoot,
        "name"
      );
      u && u.configurable && Object.defineProperty(
        e.DetermineComponentFrameRoot,
        "name",
        { value: "DetermineComponentFrameRoot" }
      );
      var n = e.DetermineComponentFrameRoot(), i = n[0], c = n[1];
      if (i && c) {
        var f = i.split(`
`), h = c.split(`
`);
        for (u = e = 0; e < f.length && !f[e].includes("DetermineComponentFrameRoot"); )
          e++;
        for (; u < h.length && !h[u].includes(
          "DetermineComponentFrameRoot"
        ); )
          u++;
        if (e === f.length || u === h.length)
          for (e = f.length - 1, u = h.length - 1; 1 <= e && 0 <= u && f[e] !== h[u]; )
            u--;
        for (; 1 <= e && 0 <= u; e--, u--)
          if (f[e] !== h[u]) {
            if (e !== 1 || u !== 1)
              do
                if (e--, u--, 0 > u || f[e] !== h[u]) {
                  var S = `
` + f[e].replace(" at new ", " at ");
                  return l.displayName && S.includes("<anonymous>") && (S = S.replace("<anonymous>", l.displayName)), S;
                }
              while (1 <= e && 0 <= u);
            break;
          }
      }
    } finally {
      Jn = !1, Error.prepareStackTrace = a;
    }
    return (a = l ? l.displayName || l.name : "") ? Ma(a) : "";
  }
  function Rv(l, t) {
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
        return wn(l.type, !1);
      case 11:
        return wn(l.type.render, !1);
      case 1:
        return wn(l.type, !0);
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
        t += Rv(l, a), a = l, l = l.return;
      while (l);
      return t;
    } catch (e) {
      return `
Error generating stack: ` + e.message + `
` + e.stack;
    }
  }
  var kn = Object.prototype.hasOwnProperty, Wn = v.unstable_scheduleCallback, $n = v.unstable_cancelCallback, xv = v.unstable_shouldYield, qv = v.unstable_requestPaint, ct = v.unstable_now, Bv = v.unstable_getCurrentPriorityLevel, bf = v.unstable_ImmediatePriority, _f = v.unstable_UserBlockingPriority, Mu = v.unstable_NormalPriority, Cv = v.unstable_LowPriority, zf = v.unstable_IdlePriority, Yv = v.log, Gv = v.unstable_setDisableYieldValue, Ne = null, ft = null;
  function ta(l) {
    if (typeof Yv == "function" && Gv(l), ft && typeof ft.setStrictMode == "function")
      try {
        ft.setStrictMode(Ne, l);
      } catch {
      }
  }
  var st = Math.clz32 ? Math.clz32 : Zv, Xv = Math.log, Qv = Math.LN2;
  function Zv(l) {
    return l >>>= 0, l === 0 ? 32 : 31 - (Xv(l) / Qv | 0) | 0;
  }
  var Ou = 256, Du = 262144, Uu = 4194304;
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
  function Nu(l, t, a) {
    var e = l.pendingLanes;
    if (e === 0) return 0;
    var u = 0, n = l.suspendedLanes, i = l.pingedLanes;
    l = l.warmLanes;
    var c = e & 134217727;
    return c !== 0 ? (e = c & ~n, e !== 0 ? u = Oa(e) : (i &= c, i !== 0 ? u = Oa(i) : a || (a = c & ~l, a !== 0 && (u = Oa(a))))) : (c = e & ~n, c !== 0 ? u = Oa(c) : i !== 0 ? u = Oa(i) : a || (a = e & ~l, a !== 0 && (u = Oa(a)))), u === 0 ? 0 : t !== 0 && t !== u && (t & n) === 0 && (n = u & -u, a = t & -t, n >= a || n === 32 && (a & 4194048) !== 0) ? t : u;
  }
  function je(l, t) {
    return (l.pendingLanes & ~(l.suspendedLanes & ~l.pingedLanes) & t) === 0;
  }
  function Lv(l, t) {
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
    var l = Uu;
    return Uu <<= 1, (Uu & 62914560) === 0 && (Uu = 4194304), l;
  }
  function Fn(l) {
    for (var t = [], a = 0; 31 > a; a++) t.push(l);
    return t;
  }
  function He(l, t) {
    l.pendingLanes |= t, t !== 268435456 && (l.suspendedLanes = 0, l.pingedLanes = 0, l.warmLanes = 0);
  }
  function Vv(l, t, a, e, u, n) {
    var i = l.pendingLanes;
    l.pendingLanes = a, l.suspendedLanes = 0, l.pingedLanes = 0, l.warmLanes = 0, l.expiredLanes &= a, l.entangledLanes &= a, l.errorRecoveryDisabledLanes &= a, l.shellSuspendCounter = 0;
    var c = l.entanglements, f = l.expirationTimes, h = l.hiddenUpdates;
    for (a = i & ~a; 0 < a; ) {
      var S = 31 - st(a), E = 1 << S;
      c[S] = 0, f[S] = -1;
      var r = h[S];
      if (r !== null)
        for (h[S] = null, S = 0; S < r.length; S++) {
          var g = r[S];
          g !== null && (g.lane &= -536870913);
        }
      a &= ~E;
    }
    e !== 0 && Tf(l, e, 0), n !== 0 && u === 0 && l.tag !== 0 && (l.suspendedLanes |= n & ~(i & ~t));
  }
  function Tf(l, t, a) {
    l.pendingLanes |= t, l.suspendedLanes &= ~t;
    var e = 31 - st(t);
    l.entangledLanes |= t, l.entanglements[e] = l.entanglements[e] | 1073741824 | a & 261930;
  }
  function Af(l, t) {
    var a = l.entangledLanes |= t;
    for (l = l.entanglements; a; ) {
      var e = 31 - st(a), u = 1 << e;
      u & t | l[e] & t && (l[e] |= t), a &= ~u;
    }
  }
  function pf(l, t) {
    var a = t & -t;
    return a = (a & 42) !== 0 ? 1 : In(a), (a & (l.suspendedLanes | t)) !== 0 ? 0 : a;
  }
  function In(l) {
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
  function Pn(l) {
    return l &= -l, 2 < l ? 8 < l ? (l & 134217727) !== 0 ? 32 : 268435456 : 8 : 2;
  }
  function Mf() {
    var l = _.p;
    return l !== 0 ? l : (l = window.event, l === void 0 ? 32 : vv(l.type));
  }
  function Of(l, t) {
    var a = _.p;
    try {
      return _.p = l, t();
    } finally {
      _.p = a;
    }
  }
  var aa = Math.random().toString(36).slice(2), Ql = "__reactFiber$" + aa, lt = "__reactProps$" + aa, Ka = "__reactContainer$" + aa, li = "__reactEvents$" + aa, Kv = "__reactListeners$" + aa, Jv = "__reactHandles$" + aa, Df = "__reactResources$" + aa, Re = "__reactMarker$" + aa;
  function ti(l) {
    delete l[Ql], delete l[lt], delete l[li], delete l[Kv], delete l[Jv];
  }
  function Ja(l) {
    var t = l[Ql];
    if (t) return t;
    for (var a = l.parentNode; a; ) {
      if (t = a[Ka] || a[Ql]) {
        if (a = t.alternate, t.child !== null || a !== null && a.child !== null)
          for (l = $d(l); l !== null; ) {
            if (a = l[Ql]) return a;
            l = $d(l);
          }
        return t;
      }
      l = a, a = l.parentNode;
    }
    return null;
  }
  function wa(l) {
    if (l = l[Ql] || l[Ka]) {
      var t = l.tag;
      if (t === 5 || t === 6 || t === 13 || t === 31 || t === 26 || t === 27 || t === 3)
        return l;
    }
    return null;
  }
  function xe(l) {
    var t = l.tag;
    if (t === 5 || t === 26 || t === 27 || t === 6) return l.stateNode;
    throw Error(s(33));
  }
  function ka(l) {
    var t = l[Df];
    return t || (t = l[Df] = { hoistableStyles: /* @__PURE__ */ new Map(), hoistableScripts: /* @__PURE__ */ new Map() }), t;
  }
  function Yl(l) {
    l[Re] = !0;
  }
  var Uf = /* @__PURE__ */ new Set(), Nf = {};
  function Da(l, t) {
    Wa(l, t), Wa(l + "Capture", t);
  }
  function Wa(l, t) {
    for (Nf[l] = t, l = 0; l < t.length; l++)
      Uf.add(t[l]);
  }
  var wv = RegExp(
    "^[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD][:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\-.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]*$"
  ), jf = {}, Hf = {};
  function kv(l) {
    return kn.call(Hf, l) ? !0 : kn.call(jf, l) ? !1 : wv.test(l) ? Hf[l] = !0 : (jf[l] = !0, !1);
  }
  function ju(l, t, a) {
    if (kv(t))
      if (a === null) l.removeAttribute(t);
      else {
        switch (typeof a) {
          case "undefined":
          case "function":
          case "symbol":
            l.removeAttribute(t);
            return;
          case "boolean":
            var e = t.toLowerCase().slice(0, 5);
            if (e !== "data-" && e !== "aria-") {
              l.removeAttribute(t);
              return;
            }
        }
        l.setAttribute(t, "" + a);
      }
  }
  function Hu(l, t, a) {
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
  function Ct(l, t, a, e) {
    if (e === null) l.removeAttribute(a);
    else {
      switch (typeof e) {
        case "undefined":
        case "function":
        case "symbol":
        case "boolean":
          l.removeAttribute(a);
          return;
      }
      l.setAttributeNS(t, a, "" + e);
    }
  }
  function bt(l) {
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
  function Wv(l, t, a) {
    var e = Object.getOwnPropertyDescriptor(
      l.constructor.prototype,
      t
    );
    if (!l.hasOwnProperty(t) && typeof e < "u" && typeof e.get == "function" && typeof e.set == "function") {
      var u = e.get, n = e.set;
      return Object.defineProperty(l, t, {
        configurable: !0,
        get: function() {
          return u.call(this);
        },
        set: function(i) {
          a = "" + i, n.call(this, i);
        }
      }), Object.defineProperty(l, t, {
        enumerable: e.enumerable
      }), {
        getValue: function() {
          return a;
        },
        setValue: function(i) {
          a = "" + i;
        },
        stopTracking: function() {
          l._valueTracker = null, delete l[t];
        }
      };
    }
  }
  function ai(l) {
    if (!l._valueTracker) {
      var t = Rf(l) ? "checked" : "value";
      l._valueTracker = Wv(
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
    var a = t.getValue(), e = "";
    return l && (e = Rf(l) ? l.checked ? "true" : "false" : l.value), l = e, l !== a ? (t.setValue(l), !0) : !1;
  }
  function Ru(l) {
    if (l = l || (typeof document < "u" ? document : void 0), typeof l > "u") return null;
    try {
      return l.activeElement || l.body;
    } catch {
      return l.body;
    }
  }
  var $v = /[\n"\\]/g;
  function _t(l) {
    return l.replace(
      $v,
      function(t) {
        return "\\" + t.charCodeAt(0).toString(16) + " ";
      }
    );
  }
  function ei(l, t, a, e, u, n, i, c) {
    l.name = "", i != null && typeof i != "function" && typeof i != "symbol" && typeof i != "boolean" ? l.type = i : l.removeAttribute("type"), t != null ? i === "number" ? (t === 0 && l.value === "" || l.value != t) && (l.value = "" + bt(t)) : l.value !== "" + bt(t) && (l.value = "" + bt(t)) : i !== "submit" && i !== "reset" || l.removeAttribute("value"), t != null ? ui(l, i, bt(t)) : a != null ? ui(l, i, bt(a)) : e != null && l.removeAttribute("value"), u == null && n != null && (l.defaultChecked = !!n), u != null && (l.checked = u && typeof u != "function" && typeof u != "symbol"), c != null && typeof c != "function" && typeof c != "symbol" && typeof c != "boolean" ? l.name = "" + bt(c) : l.removeAttribute("name");
  }
  function qf(l, t, a, e, u, n, i, c) {
    if (n != null && typeof n != "function" && typeof n != "symbol" && typeof n != "boolean" && (l.type = n), t != null || a != null) {
      if (!(n !== "submit" && n !== "reset" || t != null)) {
        ai(l);
        return;
      }
      a = a != null ? "" + bt(a) : "", t = t != null ? "" + bt(t) : a, c || t === l.value || (l.value = t), l.defaultValue = t;
    }
    e = e ?? u, e = typeof e != "function" && typeof e != "symbol" && !!e, l.checked = c ? l.checked : !!e, l.defaultChecked = !!e, i != null && typeof i != "function" && typeof i != "symbol" && typeof i != "boolean" && (l.name = i), ai(l);
  }
  function ui(l, t, a) {
    t === "number" && Ru(l.ownerDocument) === l || l.defaultValue === "" + a || (l.defaultValue = "" + a);
  }
  function $a(l, t, a, e) {
    if (l = l.options, t) {
      t = {};
      for (var u = 0; u < a.length; u++)
        t["$" + a[u]] = !0;
      for (a = 0; a < l.length; a++)
        u = t.hasOwnProperty("$" + l[a].value), l[a].selected !== u && (l[a].selected = u), u && e && (l[a].defaultSelected = !0);
    } else {
      for (a = "" + bt(a), t = null, u = 0; u < l.length; u++) {
        if (l[u].value === a) {
          l[u].selected = !0, e && (l[u].defaultSelected = !0);
          return;
        }
        t !== null || l[u].disabled || (t = l[u]);
      }
      t !== null && (t.selected = !0);
    }
  }
  function Bf(l, t, a) {
    if (t != null && (t = "" + bt(t), t !== l.value && (l.value = t), a == null)) {
      l.defaultValue !== t && (l.defaultValue = t);
      return;
    }
    l.defaultValue = a != null ? "" + bt(a) : "";
  }
  function Cf(l, t, a, e) {
    if (t == null) {
      if (e != null) {
        if (a != null) throw Error(s(92));
        if (Pl(e)) {
          if (1 < e.length) throw Error(s(93));
          e = e[0];
        }
        a = e;
      }
      a == null && (a = ""), t = a;
    }
    a = bt(t), l.defaultValue = a, e = l.textContent, e === a && e !== "" && e !== null && (l.value = e), ai(l);
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
  var Fv = new Set(
    "animationIterationCount aspectRatio borderImageOutset borderImageSlice borderImageWidth boxFlex boxFlexGroup boxOrdinalGroup columnCount columns flex flexGrow flexPositive flexShrink flexNegative flexOrder gridArea gridRow gridRowEnd gridRowSpan gridRowStart gridColumn gridColumnEnd gridColumnSpan gridColumnStart fontWeight lineClamp lineHeight opacity order orphans scale tabSize widows zIndex zoom fillOpacity floodOpacity stopOpacity strokeDasharray strokeDashoffset strokeMiterlimit strokeOpacity strokeWidth MozAnimationIterationCount MozBoxFlex MozBoxFlexGroup MozLineClamp msAnimationIterationCount msFlex msZoom msFlexGrow msFlexNegative msFlexOrder msFlexPositive msFlexShrink msGridColumn msGridColumnSpan msGridRow msGridRowSpan WebkitAnimationIterationCount WebkitBoxFlex WebKitBoxFlexGroup WebkitBoxOrdinalGroup WebkitColumnCount WebkitColumns WebkitFlex WebkitFlexGrow WebkitFlexPositive WebkitFlexShrink WebkitLineClamp".split(
      " "
    )
  );
  function Yf(l, t, a) {
    var e = t.indexOf("--") === 0;
    a == null || typeof a == "boolean" || a === "" ? e ? l.setProperty(t, "") : t === "float" ? l.cssFloat = "" : l[t] = "" : e ? l.setProperty(t, a) : typeof a != "number" || a === 0 || Fv.has(t) ? t === "float" ? l.cssFloat = a : l[t] = ("" + a).trim() : l[t] = a + "px";
  }
  function Gf(l, t, a) {
    if (t != null && typeof t != "object")
      throw Error(s(62));
    if (l = l.style, a != null) {
      for (var e in a)
        !a.hasOwnProperty(e) || t != null && t.hasOwnProperty(e) || (e.indexOf("--") === 0 ? l.setProperty(e, "") : e === "float" ? l.cssFloat = "" : l[e] = "");
      for (var u in t)
        e = t[u], t.hasOwnProperty(u) && a[u] !== e && Yf(l, u, e);
    } else
      for (var n in t)
        t.hasOwnProperty(n) && Yf(l, n, t[n]);
  }
  function ni(l) {
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
  var Iv = /* @__PURE__ */ new Map([
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
  ]), Pv = /^[\u0000-\u001F ]*j[\r\n\t]*a[\r\n\t]*v[\r\n\t]*a[\r\n\t]*s[\r\n\t]*c[\r\n\t]*r[\r\n\t]*i[\r\n\t]*p[\r\n\t]*t[\r\n\t]*:/i;
  function xu(l) {
    return Pv.test("" + l) ? "javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')" : l;
  }
  function Yt() {
  }
  var ii = null;
  function ci(l) {
    return l = l.target || l.srcElement || window, l.correspondingUseElement && (l = l.correspondingUseElement), l.nodeType === 3 ? l.parentNode : l;
  }
  var Ia = null, Pa = null;
  function Xf(l) {
    var t = wa(l);
    if (t && (l = t.stateNode)) {
      var a = l[lt] || null;
      l: switch (l = t.stateNode, t.type) {
        case "input":
          if (ei(
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
              'input[name="' + _t(
                "" + t
              ) + '"][type="radio"]'
            ), t = 0; t < a.length; t++) {
              var e = a[t];
              if (e !== l && e.form === l.form) {
                var u = e[lt] || null;
                if (!u) throw Error(s(90));
                ei(
                  e,
                  u.value,
                  u.defaultValue,
                  u.defaultValue,
                  u.checked,
                  u.defaultChecked,
                  u.type,
                  u.name
                );
              }
            }
            for (t = 0; t < a.length; t++)
              e = a[t], e.form === l.form && xf(e);
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
  var fi = !1;
  function Qf(l, t, a) {
    if (fi) return l(t, a);
    fi = !0;
    try {
      var e = l(t);
      return e;
    } finally {
      if (fi = !1, (Ia !== null || Pa !== null) && (En(), Ia && (t = Ia, l = Pa, Pa = Ia = null, Xf(t), l)))
        for (t = 0; t < l.length; t++) Xf(l[t]);
    }
  }
  function qe(l, t) {
    var a = l.stateNode;
    if (a === null) return null;
    var e = a[lt] || null;
    if (e === null) return null;
    a = e[t];
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
        (e = !e.disabled) || (l = l.type, e = !(l === "button" || l === "input" || l === "select" || l === "textarea")), l = !e;
        break l;
      default:
        l = !1;
    }
    if (l) return null;
    if (a && typeof a != "function")
      throw Error(
        s(231, t, typeof a)
      );
    return a;
  }
  var Gt = !(typeof window > "u" || typeof window.document > "u" || typeof window.document.createElement > "u"), si = !1;
  if (Gt)
    try {
      var Be = {};
      Object.defineProperty(Be, "passive", {
        get: function() {
          si = !0;
        }
      }), window.addEventListener("test", Be, Be), window.removeEventListener("test", Be, Be);
    } catch {
      si = !1;
    }
  var ea = null, oi = null, qu = null;
  function Zf() {
    if (qu) return qu;
    var l, t = oi, a = t.length, e, u = "value" in ea ? ea.value : ea.textContent, n = u.length;
    for (l = 0; l < a && t[l] === u[l]; l++) ;
    var i = a - l;
    for (e = 1; e <= i && t[a - e] === u[n - e]; e++) ;
    return qu = u.slice(l, 1 < e ? 1 - e : void 0);
  }
  function Bu(l) {
    var t = l.keyCode;
    return "charCode" in l ? (l = l.charCode, l === 0 && t === 13 && (l = 13)) : l = t, l === 10 && (l = 13), 32 <= l || l === 13 ? l : 0;
  }
  function Cu() {
    return !0;
  }
  function Lf() {
    return !1;
  }
  function tt(l) {
    function t(a, e, u, n, i) {
      this._reactName = a, this._targetInst = u, this.type = e, this.nativeEvent = n, this.target = i, this.currentTarget = null;
      for (var c in l)
        l.hasOwnProperty(c) && (a = l[c], this[c] = a ? a(n) : n[c]);
      return this.isDefaultPrevented = (n.defaultPrevented != null ? n.defaultPrevented : n.returnValue === !1) ? Cu : Lf, this.isPropagationStopped = Lf, this;
    }
    return q(t.prototype, {
      preventDefault: function() {
        this.defaultPrevented = !0;
        var a = this.nativeEvent;
        a && (a.preventDefault ? a.preventDefault() : typeof a.returnValue != "unknown" && (a.returnValue = !1), this.isDefaultPrevented = Cu);
      },
      stopPropagation: function() {
        var a = this.nativeEvent;
        a && (a.stopPropagation ? a.stopPropagation() : typeof a.cancelBubble != "unknown" && (a.cancelBubble = !0), this.isPropagationStopped = Cu);
      },
      persist: function() {
      },
      isPersistent: Cu
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
  }, Yu = tt(Ua), Ce = q({}, Ua, { view: 0, detail: 0 }), l0 = tt(Ce), di, vi, Ye, Gu = q({}, Ce, {
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
    getModifierState: mi,
    button: 0,
    buttons: 0,
    relatedTarget: function(l) {
      return l.relatedTarget === void 0 ? l.fromElement === l.srcElement ? l.toElement : l.fromElement : l.relatedTarget;
    },
    movementX: function(l) {
      return "movementX" in l ? l.movementX : (l !== Ye && (Ye && l.type === "mousemove" ? (di = l.screenX - Ye.screenX, vi = l.screenY - Ye.screenY) : vi = di = 0, Ye = l), di);
    },
    movementY: function(l) {
      return "movementY" in l ? l.movementY : vi;
    }
  }), Vf = tt(Gu), t0 = q({}, Gu, { dataTransfer: 0 }), a0 = tt(t0), e0 = q({}, Ce, { relatedTarget: 0 }), yi = tt(e0), u0 = q({}, Ua, {
    animationName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), n0 = tt(u0), i0 = q({}, Ua, {
    clipboardData: function(l) {
      return "clipboardData" in l ? l.clipboardData : window.clipboardData;
    }
  }), c0 = tt(i0), f0 = q({}, Ua, { data: 0 }), Kf = tt(f0), s0 = {
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
  }, o0 = {
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
  }, d0 = {
    Alt: "altKey",
    Control: "ctrlKey",
    Meta: "metaKey",
    Shift: "shiftKey"
  };
  function v0(l) {
    var t = this.nativeEvent;
    return t.getModifierState ? t.getModifierState(l) : (l = d0[l]) ? !!t[l] : !1;
  }
  function mi() {
    return v0;
  }
  var y0 = q({}, Ce, {
    key: function(l) {
      if (l.key) {
        var t = s0[l.key] || l.key;
        if (t !== "Unidentified") return t;
      }
      return l.type === "keypress" ? (l = Bu(l), l === 13 ? "Enter" : String.fromCharCode(l)) : l.type === "keydown" || l.type === "keyup" ? o0[l.keyCode] || "Unidentified" : "";
    },
    code: 0,
    location: 0,
    ctrlKey: 0,
    shiftKey: 0,
    altKey: 0,
    metaKey: 0,
    repeat: 0,
    locale: 0,
    getModifierState: mi,
    charCode: function(l) {
      return l.type === "keypress" ? Bu(l) : 0;
    },
    keyCode: function(l) {
      return l.type === "keydown" || l.type === "keyup" ? l.keyCode : 0;
    },
    which: function(l) {
      return l.type === "keypress" ? Bu(l) : l.type === "keydown" || l.type === "keyup" ? l.keyCode : 0;
    }
  }), m0 = tt(y0), h0 = q({}, Gu, {
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
  }), Jf = tt(h0), r0 = q({}, Ce, {
    touches: 0,
    targetTouches: 0,
    changedTouches: 0,
    altKey: 0,
    metaKey: 0,
    ctrlKey: 0,
    shiftKey: 0,
    getModifierState: mi
  }), g0 = tt(r0), S0 = q({}, Ua, {
    propertyName: 0,
    elapsedTime: 0,
    pseudoElement: 0
  }), b0 = tt(S0), _0 = q({}, Gu, {
    deltaX: function(l) {
      return "deltaX" in l ? l.deltaX : "wheelDeltaX" in l ? -l.wheelDeltaX : 0;
    },
    deltaY: function(l) {
      return "deltaY" in l ? l.deltaY : "wheelDeltaY" in l ? -l.wheelDeltaY : "wheelDelta" in l ? -l.wheelDelta : 0;
    },
    deltaZ: 0,
    deltaMode: 0
  }), z0 = tt(_0), E0 = q({}, Ua, {
    newState: 0,
    oldState: 0
  }), T0 = tt(E0), A0 = [9, 13, 27, 32], hi = Gt && "CompositionEvent" in window, Ge = null;
  Gt && "documentMode" in document && (Ge = document.documentMode);
  var p0 = Gt && "TextEvent" in window && !Ge, wf = Gt && (!hi || Ge && 8 < Ge && 11 >= Ge), kf = " ", Wf = !1;
  function $f(l, t) {
    switch (l) {
      case "keyup":
        return A0.indexOf(t.keyCode) !== -1;
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
  var le = !1;
  function M0(l, t) {
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
  function O0(l, t) {
    if (le)
      return l === "compositionend" || !hi && $f(l, t) ? (l = Zf(), qu = oi = ea = null, le = !1, l) : null;
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
  var D0 = {
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
    return t === "input" ? !!D0[l.type] : t === "textarea";
  }
  function Pf(l, t, a, e) {
    Ia ? Pa ? Pa.push(e) : Pa = [e] : Ia = e, t = Un(t, "onChange"), 0 < t.length && (a = new Yu(
      "onChange",
      "change",
      null,
      a,
      e
    ), l.push({ event: a, listeners: t }));
  }
  var Xe = null, Qe = null;
  function U0(l) {
    Bd(l, 0);
  }
  function Xu(l) {
    var t = xe(l);
    if (xf(t)) return l;
  }
  function ls(l, t) {
    if (l === "change") return t;
  }
  var ts = !1;
  if (Gt) {
    var ri;
    if (Gt) {
      var gi = "oninput" in document;
      if (!gi) {
        var as = document.createElement("div");
        as.setAttribute("oninput", "return;"), gi = typeof as.oninput == "function";
      }
      ri = gi;
    } else ri = !1;
    ts = ri && (!document.documentMode || 9 < document.documentMode);
  }
  function es() {
    Xe && (Xe.detachEvent("onpropertychange", us), Qe = Xe = null);
  }
  function us(l) {
    if (l.propertyName === "value" && Xu(Qe)) {
      var t = [];
      Pf(
        t,
        Qe,
        l,
        ci(l)
      ), Qf(U0, t);
    }
  }
  function N0(l, t, a) {
    l === "focusin" ? (es(), Xe = t, Qe = a, Xe.attachEvent("onpropertychange", us)) : l === "focusout" && es();
  }
  function j0(l) {
    if (l === "selectionchange" || l === "keyup" || l === "keydown")
      return Xu(Qe);
  }
  function H0(l, t) {
    if (l === "click") return Xu(t);
  }
  function R0(l, t) {
    if (l === "input" || l === "change")
      return Xu(t);
  }
  function x0(l, t) {
    return l === t && (l !== 0 || 1 / l === 1 / t) || l !== l && t !== t;
  }
  var ot = typeof Object.is == "function" ? Object.is : x0;
  function Ze(l, t) {
    if (ot(l, t)) return !0;
    if (typeof l != "object" || l === null || typeof t != "object" || t === null)
      return !1;
    var a = Object.keys(l), e = Object.keys(t);
    if (a.length !== e.length) return !1;
    for (e = 0; e < a.length; e++) {
      var u = a[e];
      if (!kn.call(t, u) || !ot(l[u], t[u]))
        return !1;
    }
    return !0;
  }
  function ns(l) {
    for (; l && l.firstChild; ) l = l.firstChild;
    return l;
  }
  function is(l, t) {
    var a = ns(l);
    l = 0;
    for (var e; a; ) {
      if (a.nodeType === 3) {
        if (e = l + a.textContent.length, l <= t && e >= t)
          return { node: a, offset: t - l };
        l = e;
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
  function cs(l, t) {
    return l && t ? l === t ? !0 : l && l.nodeType === 3 ? !1 : t && t.nodeType === 3 ? cs(l, t.parentNode) : "contains" in l ? l.contains(t) : l.compareDocumentPosition ? !!(l.compareDocumentPosition(t) & 16) : !1 : !1;
  }
  function fs(l) {
    l = l != null && l.ownerDocument != null && l.ownerDocument.defaultView != null ? l.ownerDocument.defaultView : window;
    for (var t = Ru(l.document); t instanceof l.HTMLIFrameElement; ) {
      try {
        var a = typeof t.contentWindow.location.href == "string";
      } catch {
        a = !1;
      }
      if (a) l = t.contentWindow;
      else break;
      t = Ru(l.document);
    }
    return t;
  }
  function Si(l) {
    var t = l && l.nodeName && l.nodeName.toLowerCase();
    return t && (t === "input" && (l.type === "text" || l.type === "search" || l.type === "tel" || l.type === "url" || l.type === "password") || t === "textarea" || l.contentEditable === "true");
  }
  var q0 = Gt && "documentMode" in document && 11 >= document.documentMode, te = null, bi = null, Le = null, _i = !1;
  function ss(l, t, a) {
    var e = a.window === a ? a.document : a.nodeType === 9 ? a : a.ownerDocument;
    _i || te == null || te !== Ru(e) || (e = te, "selectionStart" in e && Si(e) ? e = { start: e.selectionStart, end: e.selectionEnd } : (e = (e.ownerDocument && e.ownerDocument.defaultView || window).getSelection(), e = {
      anchorNode: e.anchorNode,
      anchorOffset: e.anchorOffset,
      focusNode: e.focusNode,
      focusOffset: e.focusOffset
    }), Le && Ze(Le, e) || (Le = e, e = Un(bi, "onSelect"), 0 < e.length && (t = new Yu(
      "onSelect",
      "select",
      null,
      t,
      a
    ), l.push({ event: t, listeners: e }), t.target = te)));
  }
  function Na(l, t) {
    var a = {};
    return a[l.toLowerCase()] = t.toLowerCase(), a["Webkit" + l] = "webkit" + t, a["Moz" + l] = "moz" + t, a;
  }
  var ae = {
    animationend: Na("Animation", "AnimationEnd"),
    animationiteration: Na("Animation", "AnimationIteration"),
    animationstart: Na("Animation", "AnimationStart"),
    transitionrun: Na("Transition", "TransitionRun"),
    transitionstart: Na("Transition", "TransitionStart"),
    transitioncancel: Na("Transition", "TransitionCancel"),
    transitionend: Na("Transition", "TransitionEnd")
  }, zi = {}, os = {};
  Gt && (os = document.createElement("div").style, "AnimationEvent" in window || (delete ae.animationend.animation, delete ae.animationiteration.animation, delete ae.animationstart.animation), "TransitionEvent" in window || delete ae.transitionend.transition);
  function ja(l) {
    if (zi[l]) return zi[l];
    if (!ae[l]) return l;
    var t = ae[l], a;
    for (a in t)
      if (t.hasOwnProperty(a) && a in os)
        return zi[l] = t[a];
    return l;
  }
  var ds = ja("animationend"), vs = ja("animationiteration"), ys = ja("animationstart"), B0 = ja("transitionrun"), C0 = ja("transitionstart"), Y0 = ja("transitioncancel"), ms = ja("transitionend"), hs = /* @__PURE__ */ new Map(), Ei = "abort auxClick beforeToggle cancel canPlay canPlayThrough click close contextMenu copy cut drag dragEnd dragEnter dragExit dragLeave dragOver dragStart drop durationChange emptied encrypted ended error gotPointerCapture input invalid keyDown keyPress keyUp load loadedData loadedMetadata loadStart lostPointerCapture mouseDown mouseMove mouseOut mouseOver mouseUp paste pause play playing pointerCancel pointerDown pointerMove pointerOut pointerOver pointerUp progress rateChange reset resize seeked seeking stalled submit suspend timeUpdate touchCancel touchEnd touchStart volumeChange scroll toggle touchMove waiting wheel".split(
    " "
  );
  Ei.push("scrollEnd");
  function Ut(l, t) {
    hs.set(l, t), Da(t, [l]);
  }
  var Qu = typeof reportError == "function" ? reportError : function(l) {
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
  }, zt = [], ee = 0, Ti = 0;
  function Zu() {
    for (var l = ee, t = Ti = ee = 0; t < l; ) {
      var a = zt[t];
      zt[t++] = null;
      var e = zt[t];
      zt[t++] = null;
      var u = zt[t];
      zt[t++] = null;
      var n = zt[t];
      if (zt[t++] = null, e !== null && u !== null) {
        var i = e.pending;
        i === null ? u.next = u : (u.next = i.next, i.next = u), e.pending = u;
      }
      n !== 0 && rs(a, u, n);
    }
  }
  function Lu(l, t, a, e) {
    zt[ee++] = l, zt[ee++] = t, zt[ee++] = a, zt[ee++] = e, Ti |= e, l.lanes |= e, l = l.alternate, l !== null && (l.lanes |= e);
  }
  function Ai(l, t, a, e) {
    return Lu(l, t, a, e), Vu(l);
  }
  function Ha(l, t) {
    return Lu(l, null, null, t), Vu(l);
  }
  function rs(l, t, a) {
    l.lanes |= a;
    var e = l.alternate;
    e !== null && (e.lanes |= a);
    for (var u = !1, n = l.return; n !== null; )
      n.childLanes |= a, e = n.alternate, e !== null && (e.childLanes |= a), n.tag === 22 && (l = n.stateNode, l === null || l._visibility & 1 || (u = !0)), l = n, n = n.return;
    return l.tag === 3 ? (n = l.stateNode, u && t !== null && (u = 31 - st(a), l = n.hiddenUpdates, e = l[u], e === null ? l[u] = [t] : e.push(t), t.lane = a | 536870912), n) : null;
  }
  function Vu(l) {
    if (50 < du)
      throw du = 0, Rc = null, Error(s(185));
    for (var t = l.return; t !== null; )
      l = t, t = l.return;
    return l.tag === 3 ? l.stateNode : null;
  }
  var ue = {};
  function G0(l, t, a, e) {
    this.tag = l, this.key = a, this.sibling = this.child = this.return = this.stateNode = this.type = this.elementType = null, this.index = 0, this.refCleanup = this.ref = null, this.pendingProps = t, this.dependencies = this.memoizedState = this.updateQueue = this.memoizedProps = null, this.mode = e, this.subtreeFlags = this.flags = 0, this.deletions = null, this.childLanes = this.lanes = 0, this.alternate = null;
  }
  function dt(l, t, a, e) {
    return new G0(l, t, a, e);
  }
  function pi(l) {
    return l = l.prototype, !(!l || !l.isReactComponent);
  }
  function Xt(l, t) {
    var a = l.alternate;
    return a === null ? (a = dt(
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
  function Ku(l, t, a, e, u, n) {
    var i = 0;
    if (e = l, typeof l == "function") pi(l) && (i = 1);
    else if (typeof l == "string")
      i = Vy(
        l,
        a,
        j.current
      ) ? 26 : l === "html" || l === "head" || l === "body" ? 27 : 5;
    else
      l: switch (l) {
        case Rl:
          return l = dt(31, a, t, u), l.elementType = Rl, l.lanes = n, l;
        case Ml:
          return Ra(a.children, u, n, t);
        case Wl:
          i = 8, u |= 24;
          break;
        case Ol:
          return l = dt(12, a, t, u | 2), l.elementType = Ol, l.lanes = n, l;
        case Jl:
          return l = dt(13, a, t, u), l.elementType = Jl, l.lanes = n, l;
        case Dl:
          return l = dt(19, a, t, u), l.elementType = Dl, l.lanes = n, l;
        default:
          if (typeof l == "object" && l !== null)
            switch (l.$$typeof) {
              case rl:
                i = 10;
                break l;
              case $l:
                i = 9;
                break l;
              case El:
                i = 11;
                break l;
              case Y:
                i = 14;
                break l;
              case Cl:
                i = 16, e = null;
                break l;
            }
          i = 29, a = Error(
            s(130, l === null ? "null" : typeof l, "")
          ), e = null;
      }
    return t = dt(i, a, t, u), t.elementType = l, t.type = e, t.lanes = n, t;
  }
  function Ra(l, t, a, e) {
    return l = dt(7, l, e, t), l.lanes = a, l;
  }
  function Mi(l, t, a) {
    return l = dt(6, l, null, t), l.lanes = a, l;
  }
  function Ss(l) {
    var t = dt(18, null, null, 0);
    return t.stateNode = l, t;
  }
  function Oi(l, t, a) {
    return t = dt(
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
  function Et(l, t) {
    if (typeof l == "object" && l !== null) {
      var a = bs.get(l);
      return a !== void 0 ? a : (t = {
        value: l,
        source: t,
        stack: Sf(t)
      }, bs.set(l, t), t);
    }
    return {
      value: l,
      source: t,
      stack: Sf(t)
    };
  }
  var ne = [], ie = 0, Ju = null, Ve = 0, Tt = [], At = 0, ua = null, Ht = 1, Rt = "";
  function Qt(l, t) {
    ne[ie++] = Ve, ne[ie++] = Ju, Ju = l, Ve = t;
  }
  function _s(l, t, a) {
    Tt[At++] = Ht, Tt[At++] = Rt, Tt[At++] = ua, ua = l;
    var e = Ht;
    l = Rt;
    var u = 32 - st(e) - 1;
    e &= ~(1 << u), a += 1;
    var n = 32 - st(t) + u;
    if (30 < n) {
      var i = u - u % 5;
      n = (e & (1 << i) - 1).toString(32), e >>= i, u -= i, Ht = 1 << 32 - st(t) + u | a << u | e, Rt = n + l;
    } else
      Ht = 1 << n | a << u | e, Rt = l;
  }
  function Di(l) {
    l.return !== null && (Qt(l, 1), _s(l, 1, 0));
  }
  function Ui(l) {
    for (; l === Ju; )
      Ju = ne[--ie], ne[ie] = null, Ve = ne[--ie], ne[ie] = null;
    for (; l === ua; )
      ua = Tt[--At], Tt[At] = null, Rt = Tt[--At], Tt[At] = null, Ht = Tt[--At], Tt[At] = null;
  }
  function zs(l, t) {
    Tt[At++] = Ht, Tt[At++] = Rt, Tt[At++] = ua, Ht = t.id, Rt = t.overflow, ua = l;
  }
  var Zl = null, yl = null, $ = !1, na = null, pt = !1, Ni = Error(s(519));
  function ia(l) {
    var t = Error(
      s(
        418,
        1 < arguments.length && arguments[1] !== void 0 && arguments[1] ? "text" : "HTML",
        ""
      )
    );
    throw Ke(Et(t, l)), Ni;
  }
  function Es(l) {
    var t = l.stateNode, a = l.type, e = l.memoizedProps;
    switch (t[Ql] = l, t[lt] = e, a) {
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
        for (a = 0; a < yu.length; a++)
          J(yu[a], t);
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
        J("invalid", t), qf(
          t,
          e.value,
          e.defaultValue,
          e.checked,
          e.defaultChecked,
          e.type,
          e.name,
          !0
        );
        break;
      case "select":
        J("invalid", t);
        break;
      case "textarea":
        J("invalid", t), Cf(t, e.value, e.defaultValue, e.children);
    }
    a = e.children, typeof a != "string" && typeof a != "number" && typeof a != "bigint" || t.textContent === "" + a || e.suppressHydrationWarning === !0 || Xd(t.textContent, a) ? (e.popover != null && (J("beforetoggle", t), J("toggle", t)), e.onScroll != null && J("scroll", t), e.onScrollEnd != null && J("scrollend", t), e.onClick != null && (t.onclick = Yt), t = !0) : t = !1, t || ia(l, !0);
  }
  function Ts(l) {
    for (Zl = l.return; Zl; )
      switch (Zl.tag) {
        case 5:
        case 31:
        case 13:
          pt = !1;
          return;
        case 27:
        case 3:
          pt = !0;
          return;
        default:
          Zl = Zl.return;
      }
  }
  function ce(l) {
    if (l !== Zl) return !1;
    if (!$) return Ts(l), $ = !0, !1;
    var t = l.tag, a;
    if ((a = t !== 3 && t !== 27) && ((a = t === 5) && (a = l.type, a = !(a !== "form" && a !== "button") || kc(l.type, l.memoizedProps)), a = !a), a && yl && ia(l), Ts(l), t === 13) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(s(317));
      yl = Wd(l);
    } else if (t === 31) {
      if (l = l.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(s(317));
      yl = Wd(l);
    } else
      t === 27 ? (t = yl, _a(l.type) ? (l = Pc, Pc = null, yl = l) : yl = t) : yl = Zl ? Ot(l.stateNode.nextSibling) : null;
    return !0;
  }
  function xa() {
    yl = Zl = null, $ = !1;
  }
  function ji() {
    var l = na;
    return l !== null && (nt === null ? nt = l : nt.push.apply(
      nt,
      l
    ), na = null), l;
  }
  function Ke(l) {
    na === null ? na = [l] : na.push(l);
  }
  var Hi = d(null), qa = null, Zt = null;
  function ca(l, t, a) {
    U(Hi, t._currentValue), t._currentValue = a;
  }
  function Lt(l) {
    l._currentValue = Hi.current, T(Hi);
  }
  function Ri(l, t, a) {
    for (; l !== null; ) {
      var e = l.alternate;
      if ((l.childLanes & t) !== t ? (l.childLanes |= t, e !== null && (e.childLanes |= t)) : e !== null && (e.childLanes & t) !== t && (e.childLanes |= t), l === a) break;
      l = l.return;
    }
  }
  function xi(l, t, a, e) {
    var u = l.child;
    for (u !== null && (u.return = l); u !== null; ) {
      var n = u.dependencies;
      if (n !== null) {
        var i = u.child;
        n = n.firstContext;
        l: for (; n !== null; ) {
          var c = n;
          n = u;
          for (var f = 0; f < t.length; f++)
            if (c.context === t[f]) {
              n.lanes |= a, c = n.alternate, c !== null && (c.lanes |= a), Ri(
                n.return,
                a,
                l
              ), e || (i = null);
              break l;
            }
          n = c.next;
        }
      } else if (u.tag === 18) {
        if (i = u.return, i === null) throw Error(s(341));
        i.lanes |= a, n = i.alternate, n !== null && (n.lanes |= a), Ri(i, a, l), i = null;
      } else i = u.child;
      if (i !== null) i.return = u;
      else
        for (i = u; i !== null; ) {
          if (i === l) {
            i = null;
            break;
          }
          if (u = i.sibling, u !== null) {
            u.return = i.return, i = u;
            break;
          }
          i = i.return;
        }
      u = i;
    }
  }
  function fe(l, t, a, e) {
    l = null;
    for (var u = t, n = !1; u !== null; ) {
      if (!n) {
        if ((u.flags & 524288) !== 0) n = !0;
        else if ((u.flags & 262144) !== 0) break;
      }
      if (u.tag === 10) {
        var i = u.alternate;
        if (i === null) throw Error(s(387));
        if (i = i.memoizedProps, i !== null) {
          var c = u.type;
          ot(u.pendingProps.value, i.value) || (l !== null ? l.push(c) : l = [c]);
        }
      } else if (u === il.current) {
        if (i = u.alternate, i === null) throw Error(s(387));
        i.memoizedState.memoizedState !== u.memoizedState.memoizedState && (l !== null ? l.push(Su) : l = [Su]);
      }
      u = u.return;
    }
    l !== null && xi(
      t,
      l,
      a,
      e
    ), t.flags |= 262144;
  }
  function wu(l) {
    for (l = l.firstContext; l !== null; ) {
      if (!ot(
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
  function Ll(l) {
    return As(qa, l);
  }
  function ku(l, t) {
    return qa === null && Ba(l), As(l, t);
  }
  function As(l, t) {
    var a = t._currentValue;
    if (t = { context: t, memoizedValue: a, next: null }, Zt === null) {
      if (l === null) throw Error(s(308));
      Zt = t, l.dependencies = { lanes: 0, firstContext: t }, l.flags |= 524288;
    } else Zt = Zt.next = t;
    return a;
  }
  var X0 = typeof AbortController < "u" ? AbortController : function() {
    var l = [], t = this.signal = {
      aborted: !1,
      addEventListener: function(a, e) {
        l.push(e);
      }
    };
    this.abort = function() {
      t.aborted = !0, l.forEach(function(a) {
        return a();
      });
    };
  }, Q0 = v.unstable_scheduleCallback, Z0 = v.unstable_NormalPriority, Ul = {
    $$typeof: rl,
    Consumer: null,
    Provider: null,
    _currentValue: null,
    _currentValue2: null,
    _threadCount: 0
  };
  function qi() {
    return {
      controller: new X0(),
      data: /* @__PURE__ */ new Map(),
      refCount: 0
    };
  }
  function Je(l) {
    l.refCount--, l.refCount === 0 && Q0(Z0, function() {
      l.controller.abort();
    });
  }
  var we = null, Bi = 0, se = 0, oe = null;
  function L0(l, t) {
    if (we === null) {
      var a = we = [];
      Bi = 0, se = Gc(), oe = {
        status: "pending",
        value: void 0,
        then: function(e) {
          a.push(e);
        }
      };
    }
    return Bi++, t.then(ps, ps), t;
  }
  function ps() {
    if (--Bi === 0 && we !== null) {
      oe !== null && (oe.status = "fulfilled");
      var l = we;
      we = null, se = 0, oe = null;
      for (var t = 0; t < l.length; t++) (0, l[t])();
    }
  }
  function V0(l, t) {
    var a = [], e = {
      status: "pending",
      value: null,
      reason: null,
      then: function(u) {
        a.push(u);
      }
    };
    return l.then(
      function() {
        e.status = "fulfilled", e.value = t;
        for (var u = 0; u < a.length; u++) (0, a[u])(t);
      },
      function(u) {
        for (e.status = "rejected", e.reason = u, u = 0; u < a.length; u++)
          (0, a[u])(void 0);
      }
    ), e;
  }
  var Ms = b.S;
  b.S = function(l, t) {
    od = ct(), typeof t == "object" && t !== null && typeof t.then == "function" && L0(l, t), Ms !== null && Ms(l, t);
  };
  var Ca = d(null);
  function Ci() {
    var l = Ca.current;
    return l !== null ? l : vl.pooledCache;
  }
  function Wu(l, t) {
    t === null ? U(Ca, Ca.current) : U(Ca, t.pool);
  }
  function Os() {
    var l = Ci();
    return l === null ? null : { parent: Ul._currentValue, pool: l };
  }
  var de = Error(s(460)), Yi = Error(s(474)), $u = Error(s(542)), Fu = { then: function() {
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
          if (l = vl, l !== null && 100 < l.shellSuspendCounter)
            throw Error(s(482));
          l = t, l.status = "pending", l.then(
            function(e) {
              if (t.status === "pending") {
                var u = t;
                u.status = "fulfilled", u.value = e;
              }
            },
            function(e) {
              if (t.status === "pending") {
                var u = t;
                u.status = "rejected", u.reason = e;
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
        throw Ga = t, de;
    }
  }
  function Ya(l) {
    try {
      var t = l._init;
      return t(l._payload);
    } catch (a) {
      throw a !== null && typeof a == "object" && typeof a.then == "function" ? (Ga = a, de) : a;
    }
  }
  var Ga = null;
  function Ns() {
    if (Ga === null) throw Error(s(459));
    var l = Ga;
    return Ga = null, l;
  }
  function js(l) {
    if (l === de || l === $u)
      throw Error(s(483));
  }
  var ve = null, ke = 0;
  function Iu(l) {
    var t = ke;
    return ke += 1, ve === null && (ve = []), Us(ve, l, t);
  }
  function We(l, t) {
    t = t.props.ref, l.ref = t !== void 0 ? t : null;
  }
  function Pu(l, t) {
    throw t.$$typeof === el ? Error(s(525)) : (l = Object.prototype.toString.call(t), Error(
      s(
        31,
        l === "[object Object]" ? "object with keys {" + Object.keys(t).join(", ") + "}" : l
      )
    ));
  }
  function Hs(l) {
    function t(y, o) {
      if (l) {
        var m = y.deletions;
        m === null ? (y.deletions = [o], y.flags |= 16) : m.push(o);
      }
    }
    function a(y, o) {
      if (!l) return null;
      for (; o !== null; )
        t(y, o), o = o.sibling;
      return null;
    }
    function e(y) {
      for (var o = /* @__PURE__ */ new Map(); y !== null; )
        y.key !== null ? o.set(y.key, y) : o.set(y.index, y), y = y.sibling;
      return o;
    }
    function u(y, o) {
      return y = Xt(y, o), y.index = 0, y.sibling = null, y;
    }
    function n(y, o, m) {
      return y.index = m, l ? (m = y.alternate, m !== null ? (m = m.index, m < o ? (y.flags |= 67108866, o) : m) : (y.flags |= 67108866, o)) : (y.flags |= 1048576, o);
    }
    function i(y) {
      return l && y.alternate === null && (y.flags |= 67108866), y;
    }
    function c(y, o, m, z) {
      return o === null || o.tag !== 6 ? (o = Mi(m, y.mode, z), o.return = y, o) : (o = u(o, m), o.return = y, o);
    }
    function f(y, o, m, z) {
      var B = m.type;
      return B === Ml ? S(
        y,
        o,
        m.props.children,
        z,
        m.key
      ) : o !== null && (o.elementType === B || typeof B == "object" && B !== null && B.$$typeof === Cl && Ya(B) === o.type) ? (o = u(o, m.props), We(o, m), o.return = y, o) : (o = Ku(
        m.type,
        m.key,
        m.props,
        null,
        y.mode,
        z
      ), We(o, m), o.return = y, o);
    }
    function h(y, o, m, z) {
      return o === null || o.tag !== 4 || o.stateNode.containerInfo !== m.containerInfo || o.stateNode.implementation !== m.implementation ? (o = Oi(m, y.mode, z), o.return = y, o) : (o = u(o, m.children || []), o.return = y, o);
    }
    function S(y, o, m, z, B) {
      return o === null || o.tag !== 7 ? (o = Ra(
        m,
        y.mode,
        z,
        B
      ), o.return = y, o) : (o = u(o, m), o.return = y, o);
    }
    function E(y, o, m) {
      if (typeof o == "string" && o !== "" || typeof o == "number" || typeof o == "bigint")
        return o = Mi(
          "" + o,
          y.mode,
          m
        ), o.return = y, o;
      if (typeof o == "object" && o !== null) {
        switch (o.$$typeof) {
          case Sl:
            return m = Ku(
              o.type,
              o.key,
              o.props,
              null,
              y.mode,
              m
            ), We(m, o), m.return = y, m;
          case zl:
            return o = Oi(
              o,
              y.mode,
              m
            ), o.return = y, o;
          case Cl:
            return o = Ya(o), E(y, o, m);
        }
        if (Pl(o) || ql(o))
          return o = Ra(
            o,
            y.mode,
            m,
            null
          ), o.return = y, o;
        if (typeof o.then == "function")
          return E(y, Iu(o), m);
        if (o.$$typeof === rl)
          return E(
            y,
            ku(y, o),
            m
          );
        Pu(y, o);
      }
      return null;
    }
    function r(y, o, m, z) {
      var B = o !== null ? o.key : null;
      if (typeof m == "string" && m !== "" || typeof m == "number" || typeof m == "bigint")
        return B !== null ? null : c(y, o, "" + m, z);
      if (typeof m == "object" && m !== null) {
        switch (m.$$typeof) {
          case Sl:
            return m.key === B ? f(y, o, m, z) : null;
          case zl:
            return m.key === B ? h(y, o, m, z) : null;
          case Cl:
            return m = Ya(m), r(y, o, m, z);
        }
        if (Pl(m) || ql(m))
          return B !== null ? null : S(y, o, m, z, null);
        if (typeof m.then == "function")
          return r(
            y,
            o,
            Iu(m),
            z
          );
        if (m.$$typeof === rl)
          return r(
            y,
            o,
            ku(y, m),
            z
          );
        Pu(y, m);
      }
      return null;
    }
    function g(y, o, m, z, B) {
      if (typeof z == "string" && z !== "" || typeof z == "number" || typeof z == "bigint")
        return y = y.get(m) || null, c(o, y, "" + z, B);
      if (typeof z == "object" && z !== null) {
        switch (z.$$typeof) {
          case Sl:
            return y = y.get(
              z.key === null ? m : z.key
            ) || null, f(o, y, z, B);
          case zl:
            return y = y.get(
              z.key === null ? m : z.key
            ) || null, h(o, y, z, B);
          case Cl:
            return z = Ya(z), g(
              y,
              o,
              m,
              z,
              B
            );
        }
        if (Pl(z) || ql(z))
          return y = y.get(m) || null, S(o, y, z, B, null);
        if (typeof z.then == "function")
          return g(
            y,
            o,
            m,
            Iu(z),
            B
          );
        if (z.$$typeof === rl)
          return g(
            y,
            o,
            m,
            ku(o, z),
            B
          );
        Pu(o, z);
      }
      return null;
    }
    function N(y, o, m, z) {
      for (var B = null, ll = null, H = o, Z = o = 0, k = null; H !== null && Z < m.length; Z++) {
        H.index > Z ? (k = H, H = null) : k = H.sibling;
        var tl = r(
          y,
          H,
          m[Z],
          z
        );
        if (tl === null) {
          H === null && (H = k);
          break;
        }
        l && H && tl.alternate === null && t(y, H), o = n(tl, o, Z), ll === null ? B = tl : ll.sibling = tl, ll = tl, H = k;
      }
      if (Z === m.length)
        return a(y, H), $ && Qt(y, Z), B;
      if (H === null) {
        for (; Z < m.length; Z++)
          H = E(y, m[Z], z), H !== null && (o = n(
            H,
            o,
            Z
          ), ll === null ? B = H : ll.sibling = H, ll = H);
        return $ && Qt(y, Z), B;
      }
      for (H = e(H); Z < m.length; Z++)
        k = g(
          H,
          y,
          Z,
          m[Z],
          z
        ), k !== null && (l && k.alternate !== null && H.delete(
          k.key === null ? Z : k.key
        ), o = n(
          k,
          o,
          Z
        ), ll === null ? B = k : ll.sibling = k, ll = k);
      return l && H.forEach(function(pa) {
        return t(y, pa);
      }), $ && Qt(y, Z), B;
    }
    function C(y, o, m, z) {
      if (m == null) throw Error(s(151));
      for (var B = null, ll = null, H = o, Z = o = 0, k = null, tl = m.next(); H !== null && !tl.done; Z++, tl = m.next()) {
        H.index > Z ? (k = H, H = null) : k = H.sibling;
        var pa = r(y, H, tl.value, z);
        if (pa === null) {
          H === null && (H = k);
          break;
        }
        l && H && pa.alternate === null && t(y, H), o = n(pa, o, Z), ll === null ? B = pa : ll.sibling = pa, ll = pa, H = k;
      }
      if (tl.done)
        return a(y, H), $ && Qt(y, Z), B;
      if (H === null) {
        for (; !tl.done; Z++, tl = m.next())
          tl = E(y, tl.value, z), tl !== null && (o = n(tl, o, Z), ll === null ? B = tl : ll.sibling = tl, ll = tl);
        return $ && Qt(y, Z), B;
      }
      for (H = e(H); !tl.done; Z++, tl = m.next())
        tl = g(H, y, Z, tl.value, z), tl !== null && (l && tl.alternate !== null && H.delete(tl.key === null ? Z : tl.key), o = n(tl, o, Z), ll === null ? B = tl : ll.sibling = tl, ll = tl);
      return l && H.forEach(function(tm) {
        return t(y, tm);
      }), $ && Qt(y, Z), B;
    }
    function dl(y, o, m, z) {
      if (typeof m == "object" && m !== null && m.type === Ml && m.key === null && (m = m.props.children), typeof m == "object" && m !== null) {
        switch (m.$$typeof) {
          case Sl:
            l: {
              for (var B = m.key; o !== null; ) {
                if (o.key === B) {
                  if (B = m.type, B === Ml) {
                    if (o.tag === 7) {
                      a(
                        y,
                        o.sibling
                      ), z = u(
                        o,
                        m.props.children
                      ), z.return = y, y = z;
                      break l;
                    }
                  } else if (o.elementType === B || typeof B == "object" && B !== null && B.$$typeof === Cl && Ya(B) === o.type) {
                    a(
                      y,
                      o.sibling
                    ), z = u(o, m.props), We(z, m), z.return = y, y = z;
                    break l;
                  }
                  a(y, o);
                  break;
                } else t(y, o);
                o = o.sibling;
              }
              m.type === Ml ? (z = Ra(
                m.props.children,
                y.mode,
                z,
                m.key
              ), z.return = y, y = z) : (z = Ku(
                m.type,
                m.key,
                m.props,
                null,
                y.mode,
                z
              ), We(z, m), z.return = y, y = z);
            }
            return i(y);
          case zl:
            l: {
              for (B = m.key; o !== null; ) {
                if (o.key === B)
                  if (o.tag === 4 && o.stateNode.containerInfo === m.containerInfo && o.stateNode.implementation === m.implementation) {
                    a(
                      y,
                      o.sibling
                    ), z = u(o, m.children || []), z.return = y, y = z;
                    break l;
                  } else {
                    a(y, o);
                    break;
                  }
                else t(y, o);
                o = o.sibling;
              }
              z = Oi(m, y.mode, z), z.return = y, y = z;
            }
            return i(y);
          case Cl:
            return m = Ya(m), dl(
              y,
              o,
              m,
              z
            );
        }
        if (Pl(m))
          return N(
            y,
            o,
            m,
            z
          );
        if (ql(m)) {
          if (B = ql(m), typeof B != "function") throw Error(s(150));
          return m = B.call(m), C(
            y,
            o,
            m,
            z
          );
        }
        if (typeof m.then == "function")
          return dl(
            y,
            o,
            Iu(m),
            z
          );
        if (m.$$typeof === rl)
          return dl(
            y,
            o,
            ku(y, m),
            z
          );
        Pu(y, m);
      }
      return typeof m == "string" && m !== "" || typeof m == "number" || typeof m == "bigint" ? (m = "" + m, o !== null && o.tag === 6 ? (a(y, o.sibling), z = u(o, m), z.return = y, y = z) : (a(y, o), z = Mi(m, y.mode, z), z.return = y, y = z), i(y)) : a(y, o);
    }
    return function(y, o, m, z) {
      try {
        ke = 0;
        var B = dl(
          y,
          o,
          m,
          z
        );
        return ve = null, B;
      } catch (H) {
        if (H === de || H === $u) throw H;
        var ll = dt(29, H, null, y.mode);
        return ll.lanes = z, ll.return = y, ll;
      } finally {
      }
    };
  }
  var Xa = Hs(!0), Rs = Hs(!1), fa = !1;
  function Gi(l) {
    l.updateQueue = {
      baseState: l.memoizedState,
      firstBaseUpdate: null,
      lastBaseUpdate: null,
      shared: { pending: null, lanes: 0, hiddenCallbacks: null },
      callbacks: null
    };
  }
  function Xi(l, t) {
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
  function oa(l, t, a) {
    var e = l.updateQueue;
    if (e === null) return null;
    if (e = e.shared, (ul & 2) !== 0) {
      var u = e.pending;
      return u === null ? t.next = t : (t.next = u.next, u.next = t), e.pending = t, t = Vu(l), rs(l, null, a), t;
    }
    return Lu(l, e, t, a), Vu(l);
  }
  function $e(l, t, a) {
    if (t = t.updateQueue, t !== null && (t = t.shared, (a & 4194048) !== 0)) {
      var e = t.lanes;
      e &= l.pendingLanes, a |= e, t.lanes = a, Af(l, a);
    }
  }
  function Qi(l, t) {
    var a = l.updateQueue, e = l.alternate;
    if (e !== null && (e = e.updateQueue, a === e)) {
      var u = null, n = null;
      if (a = a.firstBaseUpdate, a !== null) {
        do {
          var i = {
            lane: a.lane,
            tag: a.tag,
            payload: a.payload,
            callback: null,
            next: null
          };
          n === null ? u = n = i : n = n.next = i, a = a.next;
        } while (a !== null);
        n === null ? u = n = t : n = n.next = t;
      } else u = n = t;
      a = {
        baseState: e.baseState,
        firstBaseUpdate: u,
        lastBaseUpdate: n,
        shared: e.shared,
        callbacks: e.callbacks
      }, l.updateQueue = a;
      return;
    }
    l = a.lastBaseUpdate, l === null ? a.firstBaseUpdate = t : l.next = t, a.lastBaseUpdate = t;
  }
  var Zi = !1;
  function Fe() {
    if (Zi) {
      var l = oe;
      if (l !== null) throw l;
    }
  }
  function Ie(l, t, a, e) {
    Zi = !1;
    var u = l.updateQueue;
    fa = !1;
    var n = u.firstBaseUpdate, i = u.lastBaseUpdate, c = u.shared.pending;
    if (c !== null) {
      u.shared.pending = null;
      var f = c, h = f.next;
      f.next = null, i === null ? n = h : i.next = h, i = f;
      var S = l.alternate;
      S !== null && (S = S.updateQueue, c = S.lastBaseUpdate, c !== i && (c === null ? S.firstBaseUpdate = h : c.next = h, S.lastBaseUpdate = f));
    }
    if (n !== null) {
      var E = u.baseState;
      i = 0, S = h = f = null, c = n;
      do {
        var r = c.lane & -536870913, g = r !== c.lane;
        if (g ? (w & r) === r : (e & r) === r) {
          r !== 0 && r === se && (Zi = !0), S !== null && (S = S.next = {
            lane: 0,
            tag: c.tag,
            payload: c.payload,
            callback: null,
            next: null
          });
          l: {
            var N = l, C = c;
            r = t;
            var dl = a;
            switch (C.tag) {
              case 1:
                if (N = C.payload, typeof N == "function") {
                  E = N.call(dl, E, r);
                  break l;
                }
                E = N;
                break l;
              case 3:
                N.flags = N.flags & -65537 | 128;
              case 0:
                if (N = C.payload, r = typeof N == "function" ? N.call(dl, E, r) : N, r == null) break l;
                E = q({}, E, r);
                break l;
              case 2:
                fa = !0;
            }
          }
          r = c.callback, r !== null && (l.flags |= 64, g && (l.flags |= 8192), g = u.callbacks, g === null ? u.callbacks = [r] : g.push(r));
        } else
          g = {
            lane: r,
            tag: c.tag,
            payload: c.payload,
            callback: c.callback,
            next: null
          }, S === null ? (h = S = g, f = E) : S = S.next = g, i |= r;
        if (c = c.next, c === null) {
          if (c = u.shared.pending, c === null)
            break;
          g = c, c = g.next, g.next = null, u.lastBaseUpdate = g, u.shared.pending = null;
        }
      } while (!0);
      S === null && (f = E), u.baseState = f, u.firstBaseUpdate = h, u.lastBaseUpdate = S, n === null && (u.shared.lanes = 0), ha |= i, l.lanes = i, l.memoizedState = E;
    }
  }
  function xs(l, t) {
    if (typeof l != "function")
      throw Error(s(191, l));
    l.call(t);
  }
  function qs(l, t) {
    var a = l.callbacks;
    if (a !== null)
      for (l.callbacks = null, l = 0; l < a.length; l++)
        xs(a[l], t);
  }
  var ye = d(null), ln = d(0);
  function Bs(l, t) {
    l = It, U(ln, l), U(ye, t), It = l | t.baseLanes;
  }
  function Li() {
    U(ln, It), U(ye, ye.current);
  }
  function Vi() {
    It = ln.current, T(ye), T(ln);
  }
  var vt = d(null), Mt = null;
  function da(l) {
    var t = l.alternate;
    U(Al, Al.current & 1), U(vt, l), Mt === null && (t === null || ye.current !== null || t.memoizedState !== null) && (Mt = l);
  }
  function Ki(l) {
    U(Al, Al.current), U(vt, l), Mt === null && (Mt = l);
  }
  function Cs(l) {
    l.tag === 22 ? (U(Al, Al.current), U(vt, l), Mt === null && (Mt = l)) : va();
  }
  function va() {
    U(Al, Al.current), U(vt, vt.current);
  }
  function yt(l) {
    T(vt), Mt === l && (Mt = null), T(Al);
  }
  var Al = d(0);
  function tn(l) {
    for (var t = l; t !== null; ) {
      if (t.tag === 13) {
        var a = t.memoizedState;
        if (a !== null && (a = a.dehydrated, a === null || Fc(a) || Ic(a)))
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
  var Vt = 0, Q = null, sl = null, Nl = null, an = !1, me = !1, Qa = !1, en = 0, Pe = 0, he = null, K0 = 0;
  function bl() {
    throw Error(s(321));
  }
  function Ji(l, t) {
    if (t === null) return !1;
    for (var a = 0; a < t.length && a < l.length; a++)
      if (!ot(l[a], t[a])) return !1;
    return !0;
  }
  function wi(l, t, a, e, u, n) {
    return Vt = n, Q = t, t.memoizedState = null, t.updateQueue = null, t.lanes = 0, b.H = l === null || l.memoizedState === null ? zo : fc, Qa = !1, n = a(e, u), Qa = !1, me && (n = Gs(
      t,
      a,
      e,
      u
    )), Ys(l), n;
  }
  function Ys(l) {
    b.H = au;
    var t = sl !== null && sl.next !== null;
    if (Vt = 0, Nl = sl = Q = null, an = !1, Pe = 0, he = null, t) throw Error(s(300));
    l === null || jl || (l = l.dependencies, l !== null && wu(l) && (jl = !0));
  }
  function Gs(l, t, a, e) {
    Q = l;
    var u = 0;
    do {
      if (me && (he = null), Pe = 0, me = !1, 25 <= u) throw Error(s(301));
      if (u += 1, Nl = sl = null, l.updateQueue != null) {
        var n = l.updateQueue;
        n.lastEffect = null, n.events = null, n.stores = null, n.memoCache != null && (n.memoCache.index = 0);
      }
      b.H = Eo, n = t(a, e);
    } while (me);
    return n;
  }
  function J0() {
    var l = b.H, t = l.useState()[0];
    return t = typeof t.then == "function" ? lu(t) : t, l = l.useState()[0], (sl !== null ? sl.memoizedState : null) !== l && (Q.flags |= 1024), t;
  }
  function ki() {
    var l = en !== 0;
    return en = 0, l;
  }
  function Wi(l, t, a) {
    t.updateQueue = l.updateQueue, t.flags &= -2053, l.lanes &= ~a;
  }
  function $i(l) {
    if (an) {
      for (l = l.memoizedState; l !== null; ) {
        var t = l.queue;
        t !== null && (t.pending = null), l = l.next;
      }
      an = !1;
    }
    Vt = 0, Nl = sl = Q = null, me = !1, Pe = en = 0, he = null;
  }
  function Il() {
    var l = {
      memoizedState: null,
      baseState: null,
      baseQueue: null,
      queue: null,
      next: null
    };
    return Nl === null ? Q.memoizedState = Nl = l : Nl = Nl.next = l, Nl;
  }
  function pl() {
    if (sl === null) {
      var l = Q.alternate;
      l = l !== null ? l.memoizedState : null;
    } else l = sl.next;
    var t = Nl === null ? Q.memoizedState : Nl.next;
    if (t !== null)
      Nl = t, sl = l;
    else {
      if (l === null)
        throw Q.alternate === null ? Error(s(467)) : Error(s(310));
      sl = l, l = {
        memoizedState: sl.memoizedState,
        baseState: sl.baseState,
        baseQueue: sl.baseQueue,
        queue: sl.queue,
        next: null
      }, Nl === null ? Q.memoizedState = Nl = l : Nl = Nl.next = l;
    }
    return Nl;
  }
  function un() {
    return { lastEffect: null, events: null, stores: null, memoCache: null };
  }
  function lu(l) {
    var t = Pe;
    return Pe += 1, he === null && (he = []), l = Us(he, l, t), t = Q, (Nl === null ? t.memoizedState : Nl.next) === null && (t = t.alternate, b.H = t === null || t.memoizedState === null ? zo : fc), l;
  }
  function nn(l) {
    if (l !== null && typeof l == "object") {
      if (typeof l.then == "function") return lu(l);
      if (l.$$typeof === rl) return Ll(l);
    }
    throw Error(s(438, String(l)));
  }
  function Fi(l) {
    var t = null, a = Q.updateQueue;
    if (a !== null && (t = a.memoCache), t == null) {
      var e = Q.alternate;
      e !== null && (e = e.updateQueue, e !== null && (e = e.memoCache, e != null && (t = {
        data: e.data.map(function(u) {
          return u.slice();
        }),
        index: 0
      })));
    }
    if (t == null && (t = { data: [], index: 0 }), a === null && (a = un(), Q.updateQueue = a), a.memoCache = t, a = t.data[t.index], a === void 0)
      for (a = t.data[t.index] = Array(l), e = 0; e < l; e++)
        a[e] = wl;
    return t.index++, a;
  }
  function Kt(l, t) {
    return typeof t == "function" ? t(l) : t;
  }
  function cn(l) {
    var t = pl();
    return Ii(t, sl, l);
  }
  function Ii(l, t, a) {
    var e = l.queue;
    if (e === null) throw Error(s(311));
    e.lastRenderedReducer = a;
    var u = l.baseQueue, n = e.pending;
    if (n !== null) {
      if (u !== null) {
        var i = u.next;
        u.next = n.next, n.next = i;
      }
      t.baseQueue = u = n, e.pending = null;
    }
    if (n = l.baseState, u === null) l.memoizedState = n;
    else {
      t = u.next;
      var c = i = null, f = null, h = t, S = !1;
      do {
        var E = h.lane & -536870913;
        if (E !== h.lane ? (w & E) === E : (Vt & E) === E) {
          var r = h.revertLane;
          if (r === 0)
            f !== null && (f = f.next = {
              lane: 0,
              revertLane: 0,
              gesture: null,
              action: h.action,
              hasEagerState: h.hasEagerState,
              eagerState: h.eagerState,
              next: null
            }), E === se && (S = !0);
          else if ((Vt & r) === r) {
            h = h.next, r === se && (S = !0);
            continue;
          } else
            E = {
              lane: 0,
              revertLane: h.revertLane,
              gesture: null,
              action: h.action,
              hasEagerState: h.hasEagerState,
              eagerState: h.eagerState,
              next: null
            }, f === null ? (c = f = E, i = n) : f = f.next = E, Q.lanes |= r, ha |= r;
          E = h.action, Qa && a(n, E), n = h.hasEagerState ? h.eagerState : a(n, E);
        } else
          r = {
            lane: E,
            revertLane: h.revertLane,
            gesture: h.gesture,
            action: h.action,
            hasEagerState: h.hasEagerState,
            eagerState: h.eagerState,
            next: null
          }, f === null ? (c = f = r, i = n) : f = f.next = r, Q.lanes |= E, ha |= E;
        h = h.next;
      } while (h !== null && h !== t);
      if (f === null ? i = n : f.next = c, !ot(n, l.memoizedState) && (jl = !0, S && (a = oe, a !== null)))
        throw a;
      l.memoizedState = n, l.baseState = i, l.baseQueue = f, e.lastRenderedState = n;
    }
    return u === null && (e.lanes = 0), [l.memoizedState, e.dispatch];
  }
  function Pi(l) {
    var t = pl(), a = t.queue;
    if (a === null) throw Error(s(311));
    a.lastRenderedReducer = l;
    var e = a.dispatch, u = a.pending, n = t.memoizedState;
    if (u !== null) {
      a.pending = null;
      var i = u = u.next;
      do
        n = l(n, i.action), i = i.next;
      while (i !== u);
      ot(n, t.memoizedState) || (jl = !0), t.memoizedState = n, t.baseQueue === null && (t.baseState = n), a.lastRenderedState = n;
    }
    return [n, e];
  }
  function Xs(l, t, a) {
    var e = Q, u = pl(), n = $;
    if (n) {
      if (a === void 0) throw Error(s(407));
      a = a();
    } else a = t();
    var i = !ot(
      (sl || u).memoizedState,
      a
    );
    if (i && (u.memoizedState = a, jl = !0), u = u.queue, ac(Ls.bind(null, e, u, l), [
      l
    ]), u.getSnapshot !== t || i || Nl !== null && Nl.memoizedState.tag & 1) {
      if (e.flags |= 2048, re(
        9,
        { destroy: void 0 },
        Zs.bind(
          null,
          e,
          u,
          a,
          t
        ),
        null
      ), vl === null) throw Error(s(349));
      n || (Vt & 127) !== 0 || Qs(e, t, a);
    }
    return a;
  }
  function Qs(l, t, a) {
    l.flags |= 16384, l = { getSnapshot: t, value: a }, t = Q.updateQueue, t === null ? (t = un(), Q.updateQueue = t, t.stores = [l]) : (a = t.stores, a === null ? t.stores = [l] : a.push(l));
  }
  function Zs(l, t, a, e) {
    t.value = a, t.getSnapshot = e, Vs(t) && Ks(l);
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
      return !ot(l, a);
    } catch {
      return !0;
    }
  }
  function Ks(l) {
    var t = Ha(l, 2);
    t !== null && it(t, l, 2);
  }
  function lc(l) {
    var t = Il();
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
  function Js(l, t, a, e) {
    return l.baseState = a, Ii(
      l,
      sl,
      typeof e == "function" ? e : Kt
    );
  }
  function w0(l, t, a, e, u) {
    if (on(l)) throw Error(s(485));
    if (l = t.action, l !== null) {
      var n = {
        payload: u,
        action: l,
        next: null,
        isTransition: !0,
        status: "pending",
        value: null,
        reason: null,
        listeners: [],
        then: function(i) {
          n.listeners.push(i);
        }
      };
      b.T !== null ? a(!0) : n.isTransition = !1, e(n), a = t.pending, a === null ? (n.next = t.pending = n, ws(t, n)) : (n.next = a.next, t.pending = a.next = n);
    }
  }
  function ws(l, t) {
    var a = t.action, e = t.payload, u = l.state;
    if (t.isTransition) {
      var n = b.T, i = {};
      b.T = i;
      try {
        var c = a(u, e), f = b.S;
        f !== null && f(i, c), ks(l, t, c);
      } catch (h) {
        tc(l, t, h);
      } finally {
        n !== null && i.types !== null && (n.types = i.types), b.T = n;
      }
    } else
      try {
        n = a(u, e), ks(l, t, n);
      } catch (h) {
        tc(l, t, h);
      }
  }
  function ks(l, t, a) {
    a !== null && typeof a == "object" && typeof a.then == "function" ? a.then(
      function(e) {
        Ws(l, t, e);
      },
      function(e) {
        return tc(l, t, e);
      }
    ) : Ws(l, t, a);
  }
  function Ws(l, t, a) {
    t.status = "fulfilled", t.value = a, $s(t), l.state = a, t = l.pending, t !== null && (a = t.next, a === t ? l.pending = null : (a = a.next, t.next = a, ws(l, a)));
  }
  function tc(l, t, a) {
    var e = l.pending;
    if (l.pending = null, e !== null) {
      e = e.next;
      do
        t.status = "rejected", t.reason = a, $s(t), t = t.next;
      while (t !== e);
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
      var a = vl.formState;
      if (a !== null) {
        l: {
          var e = Q;
          if ($) {
            if (yl) {
              t: {
                for (var u = yl, n = pt; u.nodeType !== 8; ) {
                  if (!n) {
                    u = null;
                    break t;
                  }
                  if (u = Ot(
                    u.nextSibling
                  ), u === null) {
                    u = null;
                    break t;
                  }
                }
                n = u.data, u = n === "F!" || n === "F" ? u : null;
              }
              if (u) {
                yl = Ot(
                  u.nextSibling
                ), e = u.data === "F!";
                break l;
              }
            }
            ia(e);
          }
          e = !1;
        }
        e && (t = a[0]);
      }
    }
    return a = Il(), a.memoizedState = a.baseState = t, e = {
      pending: null,
      lanes: 0,
      dispatch: null,
      lastRenderedReducer: Fs,
      lastRenderedState: t
    }, a.queue = e, a = So.bind(
      null,
      Q,
      e
    ), e.dispatch = a, e = lc(!1), n = cc.bind(
      null,
      Q,
      !1,
      e.queue
    ), e = Il(), u = {
      state: t,
      dispatch: null,
      action: l,
      pending: null
    }, e.queue = u, a = w0.bind(
      null,
      Q,
      u,
      n,
      a
    ), u.dispatch = a, e.memoizedState = l, [t, a, !1];
  }
  function Ps(l) {
    var t = pl();
    return lo(t, sl, l);
  }
  function lo(l, t, a) {
    if (t = Ii(
      l,
      t,
      Fs
    )[0], l = cn(Kt)[0], typeof t == "object" && t !== null && typeof t.then == "function")
      try {
        var e = lu(t);
      } catch (i) {
        throw i === de ? $u : i;
      }
    else e = t;
    t = pl();
    var u = t.queue, n = u.dispatch;
    return a !== t.memoizedState && (Q.flags |= 2048, re(
      9,
      { destroy: void 0 },
      k0.bind(null, u, a),
      null
    )), [e, n, l];
  }
  function k0(l, t) {
    l.action = t;
  }
  function to(l) {
    var t = pl(), a = sl;
    if (a !== null)
      return lo(t, a, l);
    pl(), t = t.memoizedState, a = pl();
    var e = a.queue.dispatch;
    return a.memoizedState = l, [t, e, !1];
  }
  function re(l, t, a, e) {
    return l = { tag: l, create: a, deps: e, inst: t, next: null }, t = Q.updateQueue, t === null && (t = un(), Q.updateQueue = t), a = t.lastEffect, a === null ? t.lastEffect = l.next = l : (e = a.next, a.next = l, l.next = e, t.lastEffect = l), l;
  }
  function ao() {
    return pl().memoizedState;
  }
  function fn(l, t, a, e) {
    var u = Il();
    Q.flags |= l, u.memoizedState = re(
      1 | t,
      { destroy: void 0 },
      a,
      e === void 0 ? null : e
    );
  }
  function sn(l, t, a, e) {
    var u = pl();
    e = e === void 0 ? null : e;
    var n = u.memoizedState.inst;
    sl !== null && e !== null && Ji(e, sl.memoizedState.deps) ? u.memoizedState = re(t, n, a, e) : (Q.flags |= l, u.memoizedState = re(
      1 | t,
      n,
      a,
      e
    ));
  }
  function eo(l, t) {
    fn(8390656, 8, l, t);
  }
  function ac(l, t) {
    sn(2048, 8, l, t);
  }
  function W0(l) {
    Q.flags |= 4;
    var t = Q.updateQueue;
    if (t === null)
      t = un(), Q.updateQueue = t, t.events = [l];
    else {
      var a = t.events;
      a === null ? t.events = [l] : a.push(l);
    }
  }
  function uo(l) {
    var t = pl().memoizedState;
    return W0({ ref: t, nextImpl: l }), function() {
      if ((ul & 2) !== 0) throw Error(s(440));
      return t.impl.apply(void 0, arguments);
    };
  }
  function no(l, t) {
    return sn(4, 2, l, t);
  }
  function io(l, t) {
    return sn(4, 4, l, t);
  }
  function co(l, t) {
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
  function fo(l, t, a) {
    a = a != null ? a.concat([l]) : null, sn(4, 4, co.bind(null, t, l), a);
  }
  function ec() {
  }
  function so(l, t) {
    var a = pl();
    t = t === void 0 ? null : t;
    var e = a.memoizedState;
    return t !== null && Ji(t, e[1]) ? e[0] : (a.memoizedState = [l, t], l);
  }
  function oo(l, t) {
    var a = pl();
    t = t === void 0 ? null : t;
    var e = a.memoizedState;
    if (t !== null && Ji(t, e[1]))
      return e[0];
    if (e = l(), Qa) {
      ta(!0);
      try {
        l();
      } finally {
        ta(!1);
      }
    }
    return a.memoizedState = [e, t], e;
  }
  function uc(l, t, a) {
    return a === void 0 || (Vt & 1073741824) !== 0 && (w & 261930) === 0 ? l.memoizedState = t : (l.memoizedState = a, l = vd(), Q.lanes |= l, ha |= l, a);
  }
  function vo(l, t, a, e) {
    return ot(a, t) ? a : ye.current !== null ? (l = uc(l, a, e), ot(l, t) || (jl = !0), l) : (Vt & 42) === 0 || (Vt & 1073741824) !== 0 && (w & 261930) === 0 ? (jl = !0, l.memoizedState = a) : (l = vd(), Q.lanes |= l, ha |= l, t);
  }
  function yo(l, t, a, e, u) {
    var n = _.p;
    _.p = n !== 0 && 8 > n ? n : 8;
    var i = b.T, c = {};
    b.T = c, cc(l, !1, t, a);
    try {
      var f = u(), h = b.S;
      if (h !== null && h(c, f), f !== null && typeof f == "object" && typeof f.then == "function") {
        var S = V0(
          f,
          e
        );
        tu(
          l,
          t,
          S,
          rt(l)
        );
      } else
        tu(
          l,
          t,
          e,
          rt(l)
        );
    } catch (E) {
      tu(
        l,
        t,
        { then: function() {
        }, status: "rejected", reason: E },
        rt()
      );
    } finally {
      _.p = n, i !== null && c.types !== null && (i.types = c.types), b.T = i;
    }
  }
  function $0() {
  }
  function nc(l, t, a, e) {
    if (l.tag !== 5) throw Error(s(476));
    var u = mo(l).queue;
    yo(
      l,
      u,
      t,
      R,
      a === null ? $0 : function() {
        return ho(l), a(e);
      }
    );
  }
  function mo(l) {
    var t = l.memoizedState;
    if (t !== null) return t;
    t = {
      memoizedState: R,
      baseState: R,
      baseQueue: null,
      queue: {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: Kt,
        lastRenderedState: R
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
  function ho(l) {
    var t = mo(l);
    t.next === null && (t = l.alternate.memoizedState), tu(
      l,
      t.next.queue,
      {},
      rt()
    );
  }
  function ic() {
    return Ll(Su);
  }
  function ro() {
    return pl().memoizedState;
  }
  function go() {
    return pl().memoizedState;
  }
  function F0(l) {
    for (var t = l.return; t !== null; ) {
      switch (t.tag) {
        case 24:
        case 3:
          var a = rt();
          l = sa(a);
          var e = oa(t, l, a);
          e !== null && (it(e, t, a), $e(e, t, a)), t = { cache: qi() }, l.payload = t;
          return;
      }
      t = t.return;
    }
  }
  function I0(l, t, a) {
    var e = rt();
    a = {
      lane: e,
      revertLane: 0,
      gesture: null,
      action: a,
      hasEagerState: !1,
      eagerState: null,
      next: null
    }, on(l) ? bo(t, a) : (a = Ai(l, t, a, e), a !== null && (it(a, l, e), _o(a, t, e)));
  }
  function So(l, t, a) {
    var e = rt();
    tu(l, t, a, e);
  }
  function tu(l, t, a, e) {
    var u = {
      lane: e,
      revertLane: 0,
      gesture: null,
      action: a,
      hasEagerState: !1,
      eagerState: null,
      next: null
    };
    if (on(l)) bo(t, u);
    else {
      var n = l.alternate;
      if (l.lanes === 0 && (n === null || n.lanes === 0) && (n = t.lastRenderedReducer, n !== null))
        try {
          var i = t.lastRenderedState, c = n(i, a);
          if (u.hasEagerState = !0, u.eagerState = c, ot(c, i))
            return Lu(l, t, u, 0), vl === null && Zu(), !1;
        } catch {
        } finally {
        }
      if (a = Ai(l, t, u, e), a !== null)
        return it(a, l, e), _o(a, t, e), !0;
    }
    return !1;
  }
  function cc(l, t, a, e) {
    if (e = {
      lane: 2,
      revertLane: Gc(),
      gesture: null,
      action: e,
      hasEagerState: !1,
      eagerState: null,
      next: null
    }, on(l)) {
      if (t) throw Error(s(479));
    } else
      t = Ai(
        l,
        a,
        e,
        2
      ), t !== null && it(t, l, 2);
  }
  function on(l) {
    var t = l.alternate;
    return l === Q || t !== null && t === Q;
  }
  function bo(l, t) {
    me = an = !0;
    var a = l.pending;
    a === null ? t.next = t : (t.next = a.next, a.next = t), l.pending = t;
  }
  function _o(l, t, a) {
    if ((a & 4194048) !== 0) {
      var e = t.lanes;
      e &= l.pendingLanes, a |= e, t.lanes = a, Af(l, a);
    }
  }
  var au = {
    readContext: Ll,
    use: nn,
    useCallback: bl,
    useContext: bl,
    useEffect: bl,
    useImperativeHandle: bl,
    useLayoutEffect: bl,
    useInsertionEffect: bl,
    useMemo: bl,
    useReducer: bl,
    useRef: bl,
    useState: bl,
    useDebugValue: bl,
    useDeferredValue: bl,
    useTransition: bl,
    useSyncExternalStore: bl,
    useId: bl,
    useHostTransitionStatus: bl,
    useFormState: bl,
    useActionState: bl,
    useOptimistic: bl,
    useMemoCache: bl,
    useCacheRefresh: bl
  };
  au.useEffectEvent = bl;
  var zo = {
    readContext: Ll,
    use: nn,
    useCallback: function(l, t) {
      return Il().memoizedState = [
        l,
        t === void 0 ? null : t
      ], l;
    },
    useContext: Ll,
    useEffect: eo,
    useImperativeHandle: function(l, t, a) {
      a = a != null ? a.concat([l]) : null, fn(
        4194308,
        4,
        co.bind(null, t, l),
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
      var a = Il();
      t = t === void 0 ? null : t;
      var e = l();
      if (Qa) {
        ta(!0);
        try {
          l();
        } finally {
          ta(!1);
        }
      }
      return a.memoizedState = [e, t], e;
    },
    useReducer: function(l, t, a) {
      var e = Il();
      if (a !== void 0) {
        var u = a(t);
        if (Qa) {
          ta(!0);
          try {
            a(t);
          } finally {
            ta(!1);
          }
        }
      } else u = t;
      return e.memoizedState = e.baseState = u, l = {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: l,
        lastRenderedState: u
      }, e.queue = l, l = l.dispatch = I0.bind(
        null,
        Q,
        l
      ), [e.memoizedState, l];
    },
    useRef: function(l) {
      var t = Il();
      return l = { current: l }, t.memoizedState = l;
    },
    useState: function(l) {
      l = lc(l);
      var t = l.queue, a = So.bind(null, Q, t);
      return t.dispatch = a, [l.memoizedState, a];
    },
    useDebugValue: ec,
    useDeferredValue: function(l, t) {
      var a = Il();
      return uc(a, l, t);
    },
    useTransition: function() {
      var l = lc(!1);
      return l = yo.bind(
        null,
        Q,
        l.queue,
        !0,
        !1
      ), Il().memoizedState = l, [!1, l];
    },
    useSyncExternalStore: function(l, t, a) {
      var e = Q, u = Il();
      if ($) {
        if (a === void 0)
          throw Error(s(407));
        a = a();
      } else {
        if (a = t(), vl === null)
          throw Error(s(349));
        (w & 127) !== 0 || Qs(e, t, a);
      }
      u.memoizedState = a;
      var n = { value: a, getSnapshot: t };
      return u.queue = n, eo(Ls.bind(null, e, n, l), [
        l
      ]), e.flags |= 2048, re(
        9,
        { destroy: void 0 },
        Zs.bind(
          null,
          e,
          n,
          a,
          t
        ),
        null
      ), a;
    },
    useId: function() {
      var l = Il(), t = vl.identifierPrefix;
      if ($) {
        var a = Rt, e = Ht;
        a = (e & ~(1 << 32 - st(e) - 1)).toString(32) + a, t = "_" + t + "R_" + a, a = en++, 0 < a && (t += "H" + a.toString(32)), t += "_";
      } else
        a = K0++, t = "_" + t + "r_" + a.toString(32) + "_";
      return l.memoizedState = t;
    },
    useHostTransitionStatus: ic,
    useFormState: Is,
    useActionState: Is,
    useOptimistic: function(l) {
      var t = Il();
      t.memoizedState = t.baseState = l;
      var a = {
        pending: null,
        lanes: 0,
        dispatch: null,
        lastRenderedReducer: null,
        lastRenderedState: null
      };
      return t.queue = a, t = cc.bind(
        null,
        Q,
        !0,
        a
      ), a.dispatch = t, [l, t];
    },
    useMemoCache: Fi,
    useCacheRefresh: function() {
      return Il().memoizedState = F0.bind(
        null,
        Q
      );
    },
    useEffectEvent: function(l) {
      var t = Il(), a = { impl: l };
      return t.memoizedState = a, function() {
        if ((ul & 2) !== 0)
          throw Error(s(440));
        return a.impl.apply(void 0, arguments);
      };
    }
  }, fc = {
    readContext: Ll,
    use: nn,
    useCallback: so,
    useContext: Ll,
    useEffect: ac,
    useImperativeHandle: fo,
    useInsertionEffect: no,
    useLayoutEffect: io,
    useMemo: oo,
    useReducer: cn,
    useRef: ao,
    useState: function() {
      return cn(Kt);
    },
    useDebugValue: ec,
    useDeferredValue: function(l, t) {
      var a = pl();
      return vo(
        a,
        sl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = cn(Kt)[0], t = pl().memoizedState;
      return [
        typeof l == "boolean" ? l : lu(l),
        t
      ];
    },
    useSyncExternalStore: Xs,
    useId: ro,
    useHostTransitionStatus: ic,
    useFormState: Ps,
    useActionState: Ps,
    useOptimistic: function(l, t) {
      var a = pl();
      return Js(a, sl, l, t);
    },
    useMemoCache: Fi,
    useCacheRefresh: go
  };
  fc.useEffectEvent = uo;
  var Eo = {
    readContext: Ll,
    use: nn,
    useCallback: so,
    useContext: Ll,
    useEffect: ac,
    useImperativeHandle: fo,
    useInsertionEffect: no,
    useLayoutEffect: io,
    useMemo: oo,
    useReducer: Pi,
    useRef: ao,
    useState: function() {
      return Pi(Kt);
    },
    useDebugValue: ec,
    useDeferredValue: function(l, t) {
      var a = pl();
      return sl === null ? uc(a, l, t) : vo(
        a,
        sl.memoizedState,
        l,
        t
      );
    },
    useTransition: function() {
      var l = Pi(Kt)[0], t = pl().memoizedState;
      return [
        typeof l == "boolean" ? l : lu(l),
        t
      ];
    },
    useSyncExternalStore: Xs,
    useId: ro,
    useHostTransitionStatus: ic,
    useFormState: to,
    useActionState: to,
    useOptimistic: function(l, t) {
      var a = pl();
      return sl !== null ? Js(a, sl, l, t) : (a.baseState = l, [l, a.queue.dispatch]);
    },
    useMemoCache: Fi,
    useCacheRefresh: go
  };
  Eo.useEffectEvent = uo;
  function sc(l, t, a, e) {
    t = l.memoizedState, a = a(e, t), a = a == null ? t : q({}, t, a), l.memoizedState = a, l.lanes === 0 && (l.updateQueue.baseState = a);
  }
  var oc = {
    enqueueSetState: function(l, t, a) {
      l = l._reactInternals;
      var e = rt(), u = sa(e);
      u.payload = t, a != null && (u.callback = a), t = oa(l, u, e), t !== null && (it(t, l, e), $e(t, l, e));
    },
    enqueueReplaceState: function(l, t, a) {
      l = l._reactInternals;
      var e = rt(), u = sa(e);
      u.tag = 1, u.payload = t, a != null && (u.callback = a), t = oa(l, u, e), t !== null && (it(t, l, e), $e(t, l, e));
    },
    enqueueForceUpdate: function(l, t) {
      l = l._reactInternals;
      var a = rt(), e = sa(a);
      e.tag = 2, t != null && (e.callback = t), t = oa(l, e, a), t !== null && (it(t, l, a), $e(t, l, a));
    }
  };
  function To(l, t, a, e, u, n, i) {
    return l = l.stateNode, typeof l.shouldComponentUpdate == "function" ? l.shouldComponentUpdate(e, n, i) : t.prototype && t.prototype.isPureReactComponent ? !Ze(a, e) || !Ze(u, n) : !0;
  }
  function Ao(l, t, a, e) {
    l = t.state, typeof t.componentWillReceiveProps == "function" && t.componentWillReceiveProps(a, e), typeof t.UNSAFE_componentWillReceiveProps == "function" && t.UNSAFE_componentWillReceiveProps(a, e), t.state !== l && oc.enqueueReplaceState(t, t.state, null);
  }
  function Za(l, t) {
    var a = t;
    if ("ref" in t) {
      a = {};
      for (var e in t)
        e !== "ref" && (a[e] = t[e]);
    }
    if (l = l.defaultProps) {
      a === t && (a = q({}, a));
      for (var u in l)
        a[u] === void 0 && (a[u] = l[u]);
    }
    return a;
  }
  function po(l) {
    Qu(l);
  }
  function Mo(l) {
    console.error(l);
  }
  function Oo(l) {
    Qu(l);
  }
  function dn(l, t) {
    try {
      var a = l.onUncaughtError;
      a(t.value, { componentStack: t.stack });
    } catch (e) {
      setTimeout(function() {
        throw e;
      });
    }
  }
  function Do(l, t, a) {
    try {
      var e = l.onCaughtError;
      e(a.value, {
        componentStack: a.stack,
        errorBoundary: t.tag === 1 ? t.stateNode : null
      });
    } catch (u) {
      setTimeout(function() {
        throw u;
      });
    }
  }
  function dc(l, t, a) {
    return a = sa(a), a.tag = 3, a.payload = { element: null }, a.callback = function() {
      dn(l, t);
    }, a;
  }
  function Uo(l) {
    return l = sa(l), l.tag = 3, l;
  }
  function No(l, t, a, e) {
    var u = a.type.getDerivedStateFromError;
    if (typeof u == "function") {
      var n = e.value;
      l.payload = function() {
        return u(n);
      }, l.callback = function() {
        Do(t, a, e);
      };
    }
    var i = a.stateNode;
    i !== null && typeof i.componentDidCatch == "function" && (l.callback = function() {
      Do(t, a, e), typeof u != "function" && (ra === null ? ra = /* @__PURE__ */ new Set([this]) : ra.add(this));
      var c = e.stack;
      this.componentDidCatch(e.value, {
        componentStack: c !== null ? c : ""
      });
    });
  }
  function P0(l, t, a, e, u) {
    if (a.flags |= 32768, e !== null && typeof e == "object" && typeof e.then == "function") {
      if (t = a.alternate, t !== null && fe(
        t,
        a,
        u,
        !0
      ), a = vt.current, a !== null) {
        switch (a.tag) {
          case 31:
          case 13:
            return Mt === null ? Tn() : a.alternate === null && _l === 0 && (_l = 3), a.flags &= -257, a.flags |= 65536, a.lanes = u, e === Fu ? a.flags |= 16384 : (t = a.updateQueue, t === null ? a.updateQueue = /* @__PURE__ */ new Set([e]) : t.add(e), Bc(l, e, u)), !1;
          case 22:
            return a.flags |= 65536, e === Fu ? a.flags |= 16384 : (t = a.updateQueue, t === null ? (t = {
              transitions: null,
              markerInstances: null,
              retryQueue: /* @__PURE__ */ new Set([e])
            }, a.updateQueue = t) : (a = t.retryQueue, a === null ? t.retryQueue = /* @__PURE__ */ new Set([e]) : a.add(e)), Bc(l, e, u)), !1;
        }
        throw Error(s(435, a.tag));
      }
      return Bc(l, e, u), Tn(), !1;
    }
    if ($)
      return t = vt.current, t !== null ? ((t.flags & 65536) === 0 && (t.flags |= 256), t.flags |= 65536, t.lanes = u, e !== Ni && (l = Error(s(422), { cause: e }), Ke(Et(l, a)))) : (e !== Ni && (t = Error(s(423), {
        cause: e
      }), Ke(
        Et(t, a)
      )), l = l.current.alternate, l.flags |= 65536, u &= -u, l.lanes |= u, e = Et(e, a), u = dc(
        l.stateNode,
        e,
        u
      ), Qi(l, u), _l !== 4 && (_l = 2)), !1;
    var n = Error(s(520), { cause: e });
    if (n = Et(n, a), ou === null ? ou = [n] : ou.push(n), _l !== 4 && (_l = 2), t === null) return !0;
    e = Et(e, a), a = t;
    do {
      switch (a.tag) {
        case 3:
          return a.flags |= 65536, l = u & -u, a.lanes |= l, l = dc(a.stateNode, e, l), Qi(a, l), !1;
        case 1:
          if (t = a.type, n = a.stateNode, (a.flags & 128) === 0 && (typeof t.getDerivedStateFromError == "function" || n !== null && typeof n.componentDidCatch == "function" && (ra === null || !ra.has(n))))
            return a.flags |= 65536, u &= -u, a.lanes |= u, u = Uo(u), No(
              u,
              l,
              a,
              e
            ), Qi(a, u), !1;
      }
      a = a.return;
    } while (a !== null);
    return !1;
  }
  var vc = Error(s(461)), jl = !1;
  function Vl(l, t, a, e) {
    t.child = l === null ? Rs(t, null, a, e) : Xa(
      t,
      l.child,
      a,
      e
    );
  }
  function jo(l, t, a, e, u) {
    a = a.render;
    var n = t.ref;
    if ("ref" in e) {
      var i = {};
      for (var c in e)
        c !== "ref" && (i[c] = e[c]);
    } else i = e;
    return Ba(t), e = wi(
      l,
      t,
      a,
      i,
      n,
      u
    ), c = ki(), l !== null && !jl ? (Wi(l, t, u), Jt(l, t, u)) : ($ && c && Di(t), t.flags |= 1, Vl(l, t, e, u), t.child);
  }
  function Ho(l, t, a, e, u) {
    if (l === null) {
      var n = a.type;
      return typeof n == "function" && !pi(n) && n.defaultProps === void 0 && a.compare === null ? (t.tag = 15, t.type = n, Ro(
        l,
        t,
        n,
        e,
        u
      )) : (l = Ku(
        a.type,
        null,
        e,
        t,
        t.mode,
        u
      ), l.ref = t.ref, l.return = t, t.child = l);
    }
    if (n = l.child, !_c(l, u)) {
      var i = n.memoizedProps;
      if (a = a.compare, a = a !== null ? a : Ze, a(i, e) && l.ref === t.ref)
        return Jt(l, t, u);
    }
    return t.flags |= 1, l = Xt(n, e), l.ref = t.ref, l.return = t, t.child = l;
  }
  function Ro(l, t, a, e, u) {
    if (l !== null) {
      var n = l.memoizedProps;
      if (Ze(n, e) && l.ref === t.ref)
        if (jl = !1, t.pendingProps = e = n, _c(l, u))
          (l.flags & 131072) !== 0 && (jl = !0);
        else
          return t.lanes = l.lanes, Jt(l, t, u);
    }
    return yc(
      l,
      t,
      a,
      e,
      u
    );
  }
  function xo(l, t, a, e) {
    var u = e.children, n = l !== null ? l.memoizedState : null;
    if (l === null && t.stateNode === null && (t.stateNode = {
      _visibility: 1,
      _pendingMarkers: null,
      _retryCache: null,
      _transitions: null
    }), e.mode === "hidden") {
      if ((t.flags & 128) !== 0) {
        if (n = n !== null ? n.baseLanes | a : a, l !== null) {
          for (e = t.child = l.child, u = 0; e !== null; )
            u = u | e.lanes | e.childLanes, e = e.sibling;
          e = u & ~n;
        } else e = 0, t.child = null;
        return qo(
          l,
          t,
          n,
          a,
          e
        );
      }
      if ((a & 536870912) !== 0)
        t.memoizedState = { baseLanes: 0, cachePool: null }, l !== null && Wu(
          t,
          n !== null ? n.cachePool : null
        ), n !== null ? Bs(t, n) : Li(), Cs(t);
      else
        return e = t.lanes = 536870912, qo(
          l,
          t,
          n !== null ? n.baseLanes | a : a,
          a,
          e
        );
    } else
      n !== null ? (Wu(t, n.cachePool), Bs(t, n), va(), t.memoizedState = null) : (l !== null && Wu(t, null), Li(), va());
    return Vl(l, t, u, a), t.child;
  }
  function eu(l, t) {
    return l !== null && l.tag === 22 || t.stateNode !== null || (t.stateNode = {
      _visibility: 1,
      _pendingMarkers: null,
      _retryCache: null,
      _transitions: null
    }), t.sibling;
  }
  function qo(l, t, a, e, u) {
    var n = Ci();
    return n = n === null ? null : { parent: Ul._currentValue, pool: n }, t.memoizedState = {
      baseLanes: a,
      cachePool: n
    }, l !== null && Wu(t, null), Li(), Cs(t), l !== null && fe(l, t, e, !0), t.childLanes = u, null;
  }
  function vn(l, t) {
    return t = mn(
      { mode: t.mode, children: t.children },
      l.mode
    ), t.ref = l.ref, l.child = t, t.return = l, t;
  }
  function Bo(l, t, a) {
    return Xa(t, l.child, null, a), l = vn(t, t.pendingProps), l.flags |= 2, yt(t), t.memoizedState = null, l;
  }
  function ly(l, t, a) {
    var e = t.pendingProps, u = (t.flags & 128) !== 0;
    if (t.flags &= -129, l === null) {
      if ($) {
        if (e.mode === "hidden")
          return l = vn(t, e), t.lanes = 536870912, eu(null, l);
        if (Ki(t), (l = yl) ? (l = kd(
          l,
          pt
        ), l = l !== null && l.data === "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ua !== null ? { id: Ht, overflow: Rt } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = Ss(l), a.return = t, t.child = a, Zl = t, yl = null)) : l = null, l === null) throw ia(t);
        return t.lanes = 536870912, null;
      }
      return vn(t, e);
    }
    var n = l.memoizedState;
    if (n !== null) {
      var i = n.dehydrated;
      if (Ki(t), u)
        if (t.flags & 256)
          t.flags &= -257, t = Bo(
            l,
            t,
            a
          );
        else if (t.memoizedState !== null)
          t.child = l.child, t.flags |= 128, t = null;
        else throw Error(s(558));
      else if (jl || fe(l, t, a, !1), u = (a & l.childLanes) !== 0, jl || u) {
        if (e = vl, e !== null && (i = pf(e, a), i !== 0 && i !== n.retryLane))
          throw n.retryLane = i, Ha(l, i), it(e, l, i), vc;
        Tn(), t = Bo(
          l,
          t,
          a
        );
      } else
        l = n.treeContext, yl = Ot(i.nextSibling), Zl = t, $ = !0, na = null, pt = !1, l !== null && zs(t, l), t = vn(t, e), t.flags |= 4096;
      return t;
    }
    return l = Xt(l.child, {
      mode: e.mode,
      children: e.children
    }), l.ref = t.ref, t.child = l, l.return = t, l;
  }
  function yn(l, t) {
    var a = t.ref;
    if (a === null)
      l !== null && l.ref !== null && (t.flags |= 4194816);
    else {
      if (typeof a != "function" && typeof a != "object")
        throw Error(s(284));
      (l === null || l.ref !== a) && (t.flags |= 4194816);
    }
  }
  function yc(l, t, a, e, u) {
    return Ba(t), a = wi(
      l,
      t,
      a,
      e,
      void 0,
      u
    ), e = ki(), l !== null && !jl ? (Wi(l, t, u), Jt(l, t, u)) : ($ && e && Di(t), t.flags |= 1, Vl(l, t, a, u), t.child);
  }
  function Co(l, t, a, e, u, n) {
    return Ba(t), t.updateQueue = null, a = Gs(
      t,
      e,
      a,
      u
    ), Ys(l), e = ki(), l !== null && !jl ? (Wi(l, t, n), Jt(l, t, n)) : ($ && e && Di(t), t.flags |= 1, Vl(l, t, a, n), t.child);
  }
  function Yo(l, t, a, e, u) {
    if (Ba(t), t.stateNode === null) {
      var n = ue, i = a.contextType;
      typeof i == "object" && i !== null && (n = Ll(i)), n = new a(e, n), t.memoizedState = n.state !== null && n.state !== void 0 ? n.state : null, n.updater = oc, t.stateNode = n, n._reactInternals = t, n = t.stateNode, n.props = e, n.state = t.memoizedState, n.refs = {}, Gi(t), i = a.contextType, n.context = typeof i == "object" && i !== null ? Ll(i) : ue, n.state = t.memoizedState, i = a.getDerivedStateFromProps, typeof i == "function" && (sc(
        t,
        a,
        i,
        e
      ), n.state = t.memoizedState), typeof a.getDerivedStateFromProps == "function" || typeof n.getSnapshotBeforeUpdate == "function" || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (i = n.state, typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount(), i !== n.state && oc.enqueueReplaceState(n, n.state, null), Ie(t, e, n, u), Fe(), n.state = t.memoizedState), typeof n.componentDidMount == "function" && (t.flags |= 4194308), e = !0;
    } else if (l === null) {
      n = t.stateNode;
      var c = t.memoizedProps, f = Za(a, c);
      n.props = f;
      var h = n.context, S = a.contextType;
      i = ue, typeof S == "object" && S !== null && (i = Ll(S));
      var E = a.getDerivedStateFromProps;
      S = typeof E == "function" || typeof n.getSnapshotBeforeUpdate == "function", c = t.pendingProps !== c, S || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (c || h !== i) && Ao(
        t,
        n,
        e,
        i
      ), fa = !1;
      var r = t.memoizedState;
      n.state = r, Ie(t, e, n, u), Fe(), h = t.memoizedState, c || r !== h || fa ? (typeof E == "function" && (sc(
        t,
        a,
        E,
        e
      ), h = t.memoizedState), (f = fa || To(
        t,
        a,
        f,
        e,
        r,
        h,
        i
      )) ? (S || typeof n.UNSAFE_componentWillMount != "function" && typeof n.componentWillMount != "function" || (typeof n.componentWillMount == "function" && n.componentWillMount(), typeof n.UNSAFE_componentWillMount == "function" && n.UNSAFE_componentWillMount()), typeof n.componentDidMount == "function" && (t.flags |= 4194308)) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), t.memoizedProps = e, t.memoizedState = h), n.props = e, n.state = h, n.context = i, e = f) : (typeof n.componentDidMount == "function" && (t.flags |= 4194308), e = !1);
    } else {
      n = t.stateNode, Xi(l, t), i = t.memoizedProps, S = Za(a, i), n.props = S, E = t.pendingProps, r = n.context, h = a.contextType, f = ue, typeof h == "object" && h !== null && (f = Ll(h)), c = a.getDerivedStateFromProps, (h = typeof c == "function" || typeof n.getSnapshotBeforeUpdate == "function") || typeof n.UNSAFE_componentWillReceiveProps != "function" && typeof n.componentWillReceiveProps != "function" || (i !== E || r !== f) && Ao(
        t,
        n,
        e,
        f
      ), fa = !1, r = t.memoizedState, n.state = r, Ie(t, e, n, u), Fe();
      var g = t.memoizedState;
      i !== E || r !== g || fa || l !== null && l.dependencies !== null && wu(l.dependencies) ? (typeof c == "function" && (sc(
        t,
        a,
        c,
        e
      ), g = t.memoizedState), (S = fa || To(
        t,
        a,
        S,
        e,
        r,
        g,
        f
      ) || l !== null && l.dependencies !== null && wu(l.dependencies)) ? (h || typeof n.UNSAFE_componentWillUpdate != "function" && typeof n.componentWillUpdate != "function" || (typeof n.componentWillUpdate == "function" && n.componentWillUpdate(e, g, f), typeof n.UNSAFE_componentWillUpdate == "function" && n.UNSAFE_componentWillUpdate(
        e,
        g,
        f
      )), typeof n.componentDidUpdate == "function" && (t.flags |= 4), typeof n.getSnapshotBeforeUpdate == "function" && (t.flags |= 1024)) : (typeof n.componentDidUpdate != "function" || i === l.memoizedProps && r === l.memoizedState || (t.flags |= 4), typeof n.getSnapshotBeforeUpdate != "function" || i === l.memoizedProps && r === l.memoizedState || (t.flags |= 1024), t.memoizedProps = e, t.memoizedState = g), n.props = e, n.state = g, n.context = f, e = S) : (typeof n.componentDidUpdate != "function" || i === l.memoizedProps && r === l.memoizedState || (t.flags |= 4), typeof n.getSnapshotBeforeUpdate != "function" || i === l.memoizedProps && r === l.memoizedState || (t.flags |= 1024), e = !1);
    }
    return n = e, yn(l, t), e = (t.flags & 128) !== 0, n || e ? (n = t.stateNode, a = e && typeof a.getDerivedStateFromError != "function" ? null : n.render(), t.flags |= 1, l !== null && e ? (t.child = Xa(
      t,
      l.child,
      null,
      u
    ), t.child = Xa(
      t,
      null,
      a,
      u
    )) : Vl(l, t, a, u), t.memoizedState = n.state, l = t.child) : l = Jt(
      l,
      t,
      u
    ), l;
  }
  function Go(l, t, a, e) {
    return xa(), t.flags |= 256, Vl(l, t, a, e), t.child;
  }
  var mc = {
    dehydrated: null,
    treeContext: null,
    retryLane: 0,
    hydrationErrors: null
  };
  function hc(l) {
    return { baseLanes: l, cachePool: Os() };
  }
  function rc(l, t, a) {
    return l = l !== null ? l.childLanes & ~a : 0, t && (l |= ht), l;
  }
  function Xo(l, t, a) {
    var e = t.pendingProps, u = !1, n = (t.flags & 128) !== 0, i;
    if ((i = n) || (i = l !== null && l.memoizedState === null ? !1 : (Al.current & 2) !== 0), i && (u = !0, t.flags &= -129), i = (t.flags & 32) !== 0, t.flags &= -33, l === null) {
      if ($) {
        if (u ? da(t) : va(), (l = yl) ? (l = kd(
          l,
          pt
        ), l = l !== null && l.data !== "&" ? l : null, l !== null && (t.memoizedState = {
          dehydrated: l,
          treeContext: ua !== null ? { id: Ht, overflow: Rt } : null,
          retryLane: 536870912,
          hydrationErrors: null
        }, a = Ss(l), a.return = t, t.child = a, Zl = t, yl = null)) : l = null, l === null) throw ia(t);
        return Ic(l) ? t.lanes = 32 : t.lanes = 536870912, null;
      }
      var c = e.children;
      return e = e.fallback, u ? (va(), u = t.mode, c = mn(
        { mode: "hidden", children: c },
        u
      ), e = Ra(
        e,
        u,
        a,
        null
      ), c.return = t, e.return = t, c.sibling = e, t.child = c, e = t.child, e.memoizedState = hc(a), e.childLanes = rc(
        l,
        i,
        a
      ), t.memoizedState = mc, eu(null, e)) : (da(t), gc(t, c));
    }
    var f = l.memoizedState;
    if (f !== null && (c = f.dehydrated, c !== null)) {
      if (n)
        t.flags & 256 ? (da(t), t.flags &= -257, t = Sc(
          l,
          t,
          a
        )) : t.memoizedState !== null ? (va(), t.child = l.child, t.flags |= 128, t = null) : (va(), c = e.fallback, u = t.mode, e = mn(
          { mode: "visible", children: e.children },
          u
        ), c = Ra(
          c,
          u,
          a,
          null
        ), c.flags |= 2, e.return = t, c.return = t, e.sibling = c, t.child = e, Xa(
          t,
          l.child,
          null,
          a
        ), e = t.child, e.memoizedState = hc(a), e.childLanes = rc(
          l,
          i,
          a
        ), t.memoizedState = mc, t = eu(null, e));
      else if (da(t), Ic(c)) {
        if (i = c.nextSibling && c.nextSibling.dataset, i) var h = i.dgst;
        i = h, e = Error(s(419)), e.stack = "", e.digest = i, Ke({ value: e, source: null, stack: null }), t = Sc(
          l,
          t,
          a
        );
      } else if (jl || fe(l, t, a, !1), i = (a & l.childLanes) !== 0, jl || i) {
        if (i = vl, i !== null && (e = pf(i, a), e !== 0 && e !== f.retryLane))
          throw f.retryLane = e, Ha(l, e), it(i, l, e), vc;
        Fc(c) || Tn(), t = Sc(
          l,
          t,
          a
        );
      } else
        Fc(c) ? (t.flags |= 192, t.child = l.child, t = null) : (l = f.treeContext, yl = Ot(
          c.nextSibling
        ), Zl = t, $ = !0, na = null, pt = !1, l !== null && zs(t, l), t = gc(
          t,
          e.children
        ), t.flags |= 4096);
      return t;
    }
    return u ? (va(), c = e.fallback, u = t.mode, f = l.child, h = f.sibling, e = Xt(f, {
      mode: "hidden",
      children: e.children
    }), e.subtreeFlags = f.subtreeFlags & 65011712, h !== null ? c = Xt(
      h,
      c
    ) : (c = Ra(
      c,
      u,
      a,
      null
    ), c.flags |= 2), c.return = t, e.return = t, e.sibling = c, t.child = e, eu(null, e), e = t.child, c = l.child.memoizedState, c === null ? c = hc(a) : (u = c.cachePool, u !== null ? (f = Ul._currentValue, u = u.parent !== f ? { parent: f, pool: f } : u) : u = Os(), c = {
      baseLanes: c.baseLanes | a,
      cachePool: u
    }), e.memoizedState = c, e.childLanes = rc(
      l,
      i,
      a
    ), t.memoizedState = mc, eu(l.child, e)) : (da(t), a = l.child, l = a.sibling, a = Xt(a, {
      mode: "visible",
      children: e.children
    }), a.return = t, a.sibling = null, l !== null && (i = t.deletions, i === null ? (t.deletions = [l], t.flags |= 16) : i.push(l)), t.child = a, t.memoizedState = null, a);
  }
  function gc(l, t) {
    return t = mn(
      { mode: "visible", children: t },
      l.mode
    ), t.return = l, l.child = t;
  }
  function mn(l, t) {
    return l = dt(22, l, null, t), l.lanes = 0, l;
  }
  function Sc(l, t, a) {
    return Xa(t, l.child, null, a), l = gc(
      t,
      t.pendingProps.children
    ), l.flags |= 2, t.memoizedState = null, l;
  }
  function Qo(l, t, a) {
    l.lanes |= t;
    var e = l.alternate;
    e !== null && (e.lanes |= t), Ri(l.return, t, a);
  }
  function bc(l, t, a, e, u, n) {
    var i = l.memoizedState;
    i === null ? l.memoizedState = {
      isBackwards: t,
      rendering: null,
      renderingStartTime: 0,
      last: e,
      tail: a,
      tailMode: u,
      treeForkCount: n
    } : (i.isBackwards = t, i.rendering = null, i.renderingStartTime = 0, i.last = e, i.tail = a, i.tailMode = u, i.treeForkCount = n);
  }
  function Zo(l, t, a) {
    var e = t.pendingProps, u = e.revealOrder, n = e.tail;
    e = e.children;
    var i = Al.current, c = (i & 2) !== 0;
    if (c ? (i = i & 1 | 2, t.flags |= 128) : i &= 1, U(Al, i), Vl(l, t, e, a), e = $ ? Ve : 0, !c && l !== null && (l.flags & 128) !== 0)
      l: for (l = t.child; l !== null; ) {
        if (l.tag === 13)
          l.memoizedState !== null && Qo(l, a, t);
        else if (l.tag === 19)
          Qo(l, a, t);
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
    switch (u) {
      case "forwards":
        for (a = t.child, u = null; a !== null; )
          l = a.alternate, l !== null && tn(l) === null && (u = a), a = a.sibling;
        a = u, a === null ? (u = t.child, t.child = null) : (u = a.sibling, a.sibling = null), bc(
          t,
          !1,
          u,
          a,
          n,
          e
        );
        break;
      case "backwards":
      case "unstable_legacy-backwards":
        for (a = null, u = t.child, t.child = null; u !== null; ) {
          if (l = u.alternate, l !== null && tn(l) === null) {
            t.child = u;
            break;
          }
          l = u.sibling, u.sibling = a, a = u, u = l;
        }
        bc(
          t,
          !0,
          a,
          null,
          n,
          e
        );
        break;
      case "together":
        bc(
          t,
          !1,
          null,
          null,
          void 0,
          e
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
        if (fe(
          l,
          t,
          a,
          !1
        ), (a & t.childLanes) === 0)
          return null;
      } else return null;
    if (l !== null && t.child !== l.child)
      throw Error(s(153));
    if (t.child !== null) {
      for (l = t.child, a = Xt(l, l.pendingProps), t.child = a, a.return = t; l.sibling !== null; )
        l = l.sibling, a = a.sibling = Xt(l, l.pendingProps), a.return = t;
      a.sibling = null;
    }
    return t.child;
  }
  function _c(l, t) {
    return (l.lanes & t) !== 0 ? !0 : (l = l.dependencies, !!(l !== null && wu(l)));
  }
  function ty(l, t, a) {
    switch (t.tag) {
      case 3:
        Fl(t, t.stateNode.containerInfo), ca(t, Ul, l.memoizedState.cache), xa();
        break;
      case 27:
      case 5:
        Ue(t);
        break;
      case 4:
        Fl(t, t.stateNode.containerInfo);
        break;
      case 10:
        ca(
          t,
          t.type,
          t.memoizedProps.value
        );
        break;
      case 31:
        if (t.memoizedState !== null)
          return t.flags |= 128, Ki(t), null;
        break;
      case 13:
        var e = t.memoizedState;
        if (e !== null)
          return e.dehydrated !== null ? (da(t), t.flags |= 128, null) : (a & t.child.childLanes) !== 0 ? Xo(l, t, a) : (da(t), l = Jt(
            l,
            t,
            a
          ), l !== null ? l.sibling : null);
        da(t);
        break;
      case 19:
        var u = (l.flags & 128) !== 0;
        if (e = (a & t.childLanes) !== 0, e || (fe(
          l,
          t,
          a,
          !1
        ), e = (a & t.childLanes) !== 0), u) {
          if (e)
            return Zo(
              l,
              t,
              a
            );
          t.flags |= 128;
        }
        if (u = t.memoizedState, u !== null && (u.rendering = null, u.tail = null, u.lastEffect = null), U(Al, Al.current), e) break;
        return null;
      case 22:
        return t.lanes = 0, xo(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        ca(t, Ul, l.memoizedState.cache);
    }
    return Jt(l, t, a);
  }
  function Lo(l, t, a) {
    if (l !== null)
      if (l.memoizedProps !== t.pendingProps)
        jl = !0;
      else {
        if (!_c(l, a) && (t.flags & 128) === 0)
          return jl = !1, ty(
            l,
            t,
            a
          );
        jl = (l.flags & 131072) !== 0;
      }
    else
      jl = !1, $ && (t.flags & 1048576) !== 0 && _s(t, Ve, t.index);
    switch (t.lanes = 0, t.tag) {
      case 16:
        l: {
          var e = t.pendingProps;
          if (l = Ya(t.elementType), t.type = l, typeof l == "function")
            pi(l) ? (e = Za(l, e), t.tag = 1, t = Yo(
              null,
              t,
              l,
              e,
              a
            )) : (t.tag = 0, t = yc(
              null,
              t,
              l,
              e,
              a
            ));
          else {
            if (l != null) {
              var u = l.$$typeof;
              if (u === El) {
                t.tag = 11, t = jo(
                  null,
                  t,
                  l,
                  e,
                  a
                );
                break l;
              } else if (u === Y) {
                t.tag = 14, t = Ho(
                  null,
                  t,
                  l,
                  e,
                  a
                );
                break l;
              }
            }
            throw t = St(l) || l, Error(s(306, t, ""));
          }
        }
        return t;
      case 0:
        return yc(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 1:
        return e = t.type, u = Za(
          e,
          t.pendingProps
        ), Yo(
          l,
          t,
          e,
          u,
          a
        );
      case 3:
        l: {
          if (Fl(
            t,
            t.stateNode.containerInfo
          ), l === null) throw Error(s(387));
          e = t.pendingProps;
          var n = t.memoizedState;
          u = n.element, Xi(l, t), Ie(t, e, null, a);
          var i = t.memoizedState;
          if (e = i.cache, ca(t, Ul, e), e !== n.cache && xi(
            t,
            [Ul],
            a,
            !0
          ), Fe(), e = i.element, n.isDehydrated)
            if (n = {
              element: e,
              isDehydrated: !1,
              cache: i.cache
            }, t.updateQueue.baseState = n, t.memoizedState = n, t.flags & 256) {
              t = Go(
                l,
                t,
                e,
                a
              );
              break l;
            } else if (e !== u) {
              u = Et(
                Error(s(424)),
                t
              ), Ke(u), t = Go(
                l,
                t,
                e,
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
              for (yl = Ot(l.firstChild), Zl = t, $ = !0, na = null, pt = !0, a = Rs(
                t,
                null,
                e,
                a
              ), t.child = a; a; )
                a.flags = a.flags & -3 | 4096, a = a.sibling;
            }
          else {
            if (xa(), e === u) {
              t = Jt(
                l,
                t,
                a
              );
              break l;
            }
            Vl(l, t, e, a);
          }
          t = t.child;
        }
        return t;
      case 26:
        return yn(l, t), l === null ? (a = lv(
          t.type,
          null,
          t.pendingProps,
          null
        )) ? t.memoizedState = a : $ || (a = t.type, l = t.pendingProps, e = Nn(
          V.current
        ).createElement(a), e[Ql] = t, e[lt] = l, Kl(e, a, l), Yl(e), t.stateNode = e) : t.memoizedState = lv(
          t.type,
          l.memoizedProps,
          t.pendingProps,
          l.memoizedState
        ), null;
      case 27:
        return Ue(t), l === null && $ && (e = t.stateNode = Fd(
          t.type,
          t.pendingProps,
          V.current
        ), Zl = t, pt = !0, u = yl, _a(t.type) ? (Pc = u, yl = Ot(e.firstChild)) : yl = u), Vl(
          l,
          t,
          t.pendingProps.children,
          a
        ), yn(l, t), l === null && (t.flags |= 4194304), t.child;
      case 5:
        return l === null && $ && ((u = e = yl) && (e = jy(
          e,
          t.type,
          t.pendingProps,
          pt
        ), e !== null ? (t.stateNode = e, Zl = t, yl = Ot(e.firstChild), pt = !1, u = !0) : u = !1), u || ia(t)), Ue(t), u = t.type, n = t.pendingProps, i = l !== null ? l.memoizedProps : null, e = n.children, kc(u, n) ? e = null : i !== null && kc(u, i) && (t.flags |= 32), t.memoizedState !== null && (u = wi(
          l,
          t,
          J0,
          null,
          null,
          a
        ), Su._currentValue = u), yn(l, t), Vl(l, t, e, a), t.child;
      case 6:
        return l === null && $ && ((l = a = yl) && (a = Hy(
          a,
          t.pendingProps,
          pt
        ), a !== null ? (t.stateNode = a, Zl = t, yl = null, l = !0) : l = !1), l || ia(t)), null;
      case 13:
        return Xo(l, t, a);
      case 4:
        return Fl(
          t,
          t.stateNode.containerInfo
        ), e = t.pendingProps, l === null ? t.child = Xa(
          t,
          null,
          e,
          a
        ) : Vl(l, t, e, a), t.child;
      case 11:
        return jo(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 7:
        return Vl(
          l,
          t,
          t.pendingProps,
          a
        ), t.child;
      case 8:
        return Vl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 12:
        return Vl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 10:
        return e = t.pendingProps, ca(t, t.type, e.value), Vl(l, t, e.children, a), t.child;
      case 9:
        return u = t.type._context, e = t.pendingProps.children, Ba(t), u = Ll(u), e = e(u), t.flags |= 1, Vl(l, t, e, a), t.child;
      case 14:
        return Ho(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 15:
        return Ro(
          l,
          t,
          t.type,
          t.pendingProps,
          a
        );
      case 19:
        return Zo(l, t, a);
      case 31:
        return ly(l, t, a);
      case 22:
        return xo(
          l,
          t,
          a,
          t.pendingProps
        );
      case 24:
        return Ba(t), e = Ll(Ul), l === null ? (u = Ci(), u === null && (u = vl, n = qi(), u.pooledCache = n, n.refCount++, n !== null && (u.pooledCacheLanes |= a), u = n), t.memoizedState = { parent: e, cache: u }, Gi(t), ca(t, Ul, u)) : ((l.lanes & a) !== 0 && (Xi(l, t), Ie(t, null, null, a), Fe()), u = l.memoizedState, n = t.memoizedState, u.parent !== e ? (u = { parent: e, cache: e }, t.memoizedState = u, t.lanes === 0 && (t.memoizedState = t.updateQueue.baseState = u), ca(t, Ul, e)) : (e = n.cache, ca(t, Ul, e), e !== u.cache && xi(
          t,
          [Ul],
          a,
          !0
        ))), Vl(
          l,
          t,
          t.pendingProps.children,
          a
        ), t.child;
      case 29:
        throw t.pendingProps;
    }
    throw Error(s(156, t.tag));
  }
  function wt(l) {
    l.flags |= 4;
  }
  function zc(l, t, a, e, u) {
    if ((t = (l.mode & 32) !== 0) && (t = !1), t) {
      if (l.flags |= 16777216, (u & 335544128) === u)
        if (l.stateNode.complete) l.flags |= 8192;
        else if (rd()) l.flags |= 8192;
        else
          throw Ga = Fu, Yi;
    } else l.flags &= -16777217;
  }
  function Vo(l, t) {
    if (t.type !== "stylesheet" || (t.state.loading & 4) !== 0)
      l.flags &= -16777217;
    else if (l.flags |= 16777216, !nv(t))
      if (rd()) l.flags |= 8192;
      else
        throw Ga = Fu, Yi;
  }
  function hn(l, t) {
    t !== null && (l.flags |= 4), l.flags & 16384 && (t = l.tag !== 22 ? Ef() : 536870912, l.lanes |= t, _e |= t);
  }
  function uu(l, t) {
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
          for (var e = null; a !== null; )
            a.alternate !== null && (e = a), a = a.sibling;
          e === null ? t || l.tail === null ? l.tail = null : l.tail.sibling = null : e.sibling = null;
      }
  }
  function ml(l) {
    var t = l.alternate !== null && l.alternate.child === l.child, a = 0, e = 0;
    if (t)
      for (var u = l.child; u !== null; )
        a |= u.lanes | u.childLanes, e |= u.subtreeFlags & 65011712, e |= u.flags & 65011712, u.return = l, u = u.sibling;
    else
      for (u = l.child; u !== null; )
        a |= u.lanes | u.childLanes, e |= u.subtreeFlags, e |= u.flags, u.return = l, u = u.sibling;
    return l.subtreeFlags |= e, l.childLanes = a, t;
  }
  function ay(l, t, a) {
    var e = t.pendingProps;
    switch (Ui(t), t.tag) {
      case 16:
      case 15:
      case 0:
      case 11:
      case 7:
      case 8:
      case 12:
      case 9:
      case 14:
        return ml(t), null;
      case 1:
        return ml(t), null;
      case 3:
        return a = t.stateNode, e = null, l !== null && (e = l.memoizedState.cache), t.memoizedState.cache !== e && (t.flags |= 2048), Lt(Ul), Tl(), a.pendingContext && (a.context = a.pendingContext, a.pendingContext = null), (l === null || l.child === null) && (ce(t) ? wt(t) : l === null || l.memoizedState.isDehydrated && (t.flags & 256) === 0 || (t.flags |= 1024, ji())), ml(t), null;
      case 26:
        var u = t.type, n = t.memoizedState;
        return l === null ? (wt(t), n !== null ? (ml(t), Vo(t, n)) : (ml(t), zc(
          t,
          u,
          null,
          e,
          a
        ))) : n ? n !== l.memoizedState ? (wt(t), ml(t), Vo(t, n)) : (ml(t), t.flags &= -16777217) : (l = l.memoizedProps, l !== e && wt(t), ml(t), zc(
          t,
          u,
          l,
          e,
          a
        )), null;
      case 27:
        if (pu(t), a = V.current, u = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== e && wt(t);
        else {
          if (!e) {
            if (t.stateNode === null)
              throw Error(s(166));
            return ml(t), null;
          }
          l = j.current, ce(t) ? Es(t) : (l = Fd(u, e, a), t.stateNode = l, wt(t));
        }
        return ml(t), null;
      case 5:
        if (pu(t), u = t.type, l !== null && t.stateNode != null)
          l.memoizedProps !== e && wt(t);
        else {
          if (!e) {
            if (t.stateNode === null)
              throw Error(s(166));
            return ml(t), null;
          }
          if (n = j.current, ce(t))
            Es(t);
          else {
            var i = Nn(
              V.current
            );
            switch (n) {
              case 1:
                n = i.createElementNS(
                  "http://www.w3.org/2000/svg",
                  u
                );
                break;
              case 2:
                n = i.createElementNS(
                  "http://www.w3.org/1998/Math/MathML",
                  u
                );
                break;
              default:
                switch (u) {
                  case "svg":
                    n = i.createElementNS(
                      "http://www.w3.org/2000/svg",
                      u
                    );
                    break;
                  case "math":
                    n = i.createElementNS(
                      "http://www.w3.org/1998/Math/MathML",
                      u
                    );
                    break;
                  case "script":
                    n = i.createElement("div"), n.innerHTML = "<script><\/script>", n = n.removeChild(
                      n.firstChild
                    );
                    break;
                  case "select":
                    n = typeof e.is == "string" ? i.createElement("select", {
                      is: e.is
                    }) : i.createElement("select"), e.multiple ? n.multiple = !0 : e.size && (n.size = e.size);
                    break;
                  default:
                    n = typeof e.is == "string" ? i.createElement(u, { is: e.is }) : i.createElement(u);
                }
            }
            n[Ql] = t, n[lt] = e;
            l: for (i = t.child; i !== null; ) {
              if (i.tag === 5 || i.tag === 6)
                n.appendChild(i.stateNode);
              else if (i.tag !== 4 && i.tag !== 27 && i.child !== null) {
                i.child.return = i, i = i.child;
                continue;
              }
              if (i === t) break l;
              for (; i.sibling === null; ) {
                if (i.return === null || i.return === t)
                  break l;
                i = i.return;
              }
              i.sibling.return = i.return, i = i.sibling;
            }
            t.stateNode = n;
            l: switch (Kl(n, u, e), u) {
              case "button":
              case "input":
              case "select":
              case "textarea":
                e = !!e.autoFocus;
                break l;
              case "img":
                e = !0;
                break l;
              default:
                e = !1;
            }
            e && wt(t);
          }
        }
        return ml(t), zc(
          t,
          t.type,
          l === null ? null : l.memoizedProps,
          t.pendingProps,
          a
        ), null;
      case 6:
        if (l && t.stateNode != null)
          l.memoizedProps !== e && wt(t);
        else {
          if (typeof e != "string" && t.stateNode === null)
            throw Error(s(166));
          if (l = V.current, ce(t)) {
            if (l = t.stateNode, a = t.memoizedProps, e = null, u = Zl, u !== null)
              switch (u.tag) {
                case 27:
                case 5:
                  e = u.memoizedProps;
              }
            l[Ql] = t, l = !!(l.nodeValue === a || e !== null && e.suppressHydrationWarning === !0 || Xd(l.nodeValue, a)), l || ia(t, !0);
          } else
            l = Nn(l).createTextNode(
              e
            ), l[Ql] = t, t.stateNode = l;
        }
        return ml(t), null;
      case 31:
        if (a = t.memoizedState, l === null || l.memoizedState !== null) {
          if (e = ce(t), a !== null) {
            if (l === null) {
              if (!e) throw Error(s(318));
              if (l = t.memoizedState, l = l !== null ? l.dehydrated : null, !l) throw Error(s(557));
              l[Ql] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            ml(t), l = !1;
          } else
            a = ji(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = a), l = !0;
          if (!l)
            return t.flags & 256 ? (yt(t), t) : (yt(t), null);
          if ((t.flags & 128) !== 0)
            throw Error(s(558));
        }
        return ml(t), null;
      case 13:
        if (e = t.memoizedState, l === null || l.memoizedState !== null && l.memoizedState.dehydrated !== null) {
          if (u = ce(t), e !== null && e.dehydrated !== null) {
            if (l === null) {
              if (!u) throw Error(s(318));
              if (u = t.memoizedState, u = u !== null ? u.dehydrated : null, !u) throw Error(s(317));
              u[Ql] = t;
            } else
              xa(), (t.flags & 128) === 0 && (t.memoizedState = null), t.flags |= 4;
            ml(t), u = !1;
          } else
            u = ji(), l !== null && l.memoizedState !== null && (l.memoizedState.hydrationErrors = u), u = !0;
          if (!u)
            return t.flags & 256 ? (yt(t), t) : (yt(t), null);
        }
        return yt(t), (t.flags & 128) !== 0 ? (t.lanes = a, t) : (a = e !== null, l = l !== null && l.memoizedState !== null, a && (e = t.child, u = null, e.alternate !== null && e.alternate.memoizedState !== null && e.alternate.memoizedState.cachePool !== null && (u = e.alternate.memoizedState.cachePool.pool), n = null, e.memoizedState !== null && e.memoizedState.cachePool !== null && (n = e.memoizedState.cachePool.pool), n !== u && (e.flags |= 2048)), a !== l && a && (t.child.flags |= 8192), hn(t, t.updateQueue), ml(t), null);
      case 4:
        return Tl(), l === null && Lc(t.stateNode.containerInfo), ml(t), null;
      case 10:
        return Lt(t.type), ml(t), null;
      case 19:
        if (T(Al), e = t.memoizedState, e === null) return ml(t), null;
        if (u = (t.flags & 128) !== 0, n = e.rendering, n === null)
          if (u) uu(e, !1);
          else {
            if (_l !== 0 || l !== null && (l.flags & 128) !== 0)
              for (l = t.child; l !== null; ) {
                if (n = tn(l), n !== null) {
                  for (t.flags |= 128, uu(e, !1), l = n.updateQueue, t.updateQueue = l, hn(t, l), t.subtreeFlags = 0, l = a, a = t.child; a !== null; )
                    gs(a, l), a = a.sibling;
                  return U(
                    Al,
                    Al.current & 1 | 2
                  ), $ && Qt(t, e.treeForkCount), t.child;
                }
                l = l.sibling;
              }
            e.tail !== null && ct() > _n && (t.flags |= 128, u = !0, uu(e, !1), t.lanes = 4194304);
          }
        else {
          if (!u)
            if (l = tn(n), l !== null) {
              if (t.flags |= 128, u = !0, l = l.updateQueue, t.updateQueue = l, hn(t, l), uu(e, !0), e.tail === null && e.tailMode === "hidden" && !n.alternate && !$)
                return ml(t), null;
            } else
              2 * ct() - e.renderingStartTime > _n && a !== 536870912 && (t.flags |= 128, u = !0, uu(e, !1), t.lanes = 4194304);
          e.isBackwards ? (n.sibling = t.child, t.child = n) : (l = e.last, l !== null ? l.sibling = n : t.child = n, e.last = n);
        }
        return e.tail !== null ? (l = e.tail, e.rendering = l, e.tail = l.sibling, e.renderingStartTime = ct(), l.sibling = null, a = Al.current, U(
          Al,
          u ? a & 1 | 2 : a & 1
        ), $ && Qt(t, e.treeForkCount), l) : (ml(t), null);
      case 22:
      case 23:
        return yt(t), Vi(), e = t.memoizedState !== null, l !== null ? l.memoizedState !== null !== e && (t.flags |= 8192) : e && (t.flags |= 8192), e ? (a & 536870912) !== 0 && (t.flags & 128) === 0 && (ml(t), t.subtreeFlags & 6 && (t.flags |= 8192)) : ml(t), a = t.updateQueue, a !== null && hn(t, a.retryQueue), a = null, l !== null && l.memoizedState !== null && l.memoizedState.cachePool !== null && (a = l.memoizedState.cachePool.pool), e = null, t.memoizedState !== null && t.memoizedState.cachePool !== null && (e = t.memoizedState.cachePool.pool), e !== a && (t.flags |= 2048), l !== null && T(Ca), null;
      case 24:
        return a = null, l !== null && (a = l.memoizedState.cache), t.memoizedState.cache !== a && (t.flags |= 2048), Lt(Ul), ml(t), null;
      case 25:
        return null;
      case 30:
        return null;
    }
    throw Error(s(156, t.tag));
  }
  function ey(l, t) {
    switch (Ui(t), t.tag) {
      case 1:
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 3:
        return Lt(Ul), Tl(), l = t.flags, (l & 65536) !== 0 && (l & 128) === 0 ? (t.flags = l & -65537 | 128, t) : null;
      case 26:
      case 27:
      case 5:
        return pu(t), null;
      case 31:
        if (t.memoizedState !== null) {
          if (yt(t), t.alternate === null)
            throw Error(s(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 13:
        if (yt(t), l = t.memoizedState, l !== null && l.dehydrated !== null) {
          if (t.alternate === null)
            throw Error(s(340));
          xa();
        }
        return l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 19:
        return T(Al), null;
      case 4:
        return Tl(), null;
      case 10:
        return Lt(t.type), null;
      case 22:
      case 23:
        return yt(t), Vi(), l !== null && T(Ca), l = t.flags, l & 65536 ? (t.flags = l & -65537 | 128, t) : null;
      case 24:
        return Lt(Ul), null;
      case 25:
        return null;
      default:
        return null;
    }
  }
  function Ko(l, t) {
    switch (Ui(t), t.tag) {
      case 3:
        Lt(Ul), Tl();
        break;
      case 26:
      case 27:
      case 5:
        pu(t);
        break;
      case 4:
        Tl();
        break;
      case 31:
        t.memoizedState !== null && yt(t);
        break;
      case 13:
        yt(t);
        break;
      case 19:
        T(Al);
        break;
      case 10:
        Lt(t.type);
        break;
      case 22:
      case 23:
        yt(t), Vi(), l !== null && T(Ca);
        break;
      case 24:
        Lt(Ul);
    }
  }
  function nu(l, t) {
    try {
      var a = t.updateQueue, e = a !== null ? a.lastEffect : null;
      if (e !== null) {
        var u = e.next;
        a = u;
        do {
          if ((a.tag & l) === l) {
            e = void 0;
            var n = a.create, i = a.inst;
            e = n(), i.destroy = e;
          }
          a = a.next;
        } while (a !== u);
      }
    } catch (c) {
      fl(t, t.return, c);
    }
  }
  function ya(l, t, a) {
    try {
      var e = t.updateQueue, u = e !== null ? e.lastEffect : null;
      if (u !== null) {
        var n = u.next;
        e = n;
        do {
          if ((e.tag & l) === l) {
            var i = e.inst, c = i.destroy;
            if (c !== void 0) {
              i.destroy = void 0, u = t;
              var f = a, h = c;
              try {
                h();
              } catch (S) {
                fl(
                  u,
                  f,
                  S
                );
              }
            }
          }
          e = e.next;
        } while (e !== n);
      }
    } catch (S) {
      fl(t, t.return, S);
    }
  }
  function Jo(l) {
    var t = l.updateQueue;
    if (t !== null) {
      var a = l.stateNode;
      try {
        qs(t, a);
      } catch (e) {
        fl(l, l.return, e);
      }
    }
  }
  function wo(l, t, a) {
    a.props = Za(
      l.type,
      l.memoizedProps
    ), a.state = l.memoizedState;
    try {
      a.componentWillUnmount();
    } catch (e) {
      fl(l, t, e);
    }
  }
  function iu(l, t) {
    try {
      var a = l.ref;
      if (a !== null) {
        switch (l.tag) {
          case 26:
          case 27:
          case 5:
            var e = l.stateNode;
            break;
          case 30:
            e = l.stateNode;
            break;
          default:
            e = l.stateNode;
        }
        typeof a == "function" ? l.refCleanup = a(e) : a.current = e;
      }
    } catch (u) {
      fl(l, t, u);
    }
  }
  function xt(l, t) {
    var a = l.ref, e = l.refCleanup;
    if (a !== null)
      if (typeof e == "function")
        try {
          e();
        } catch (u) {
          fl(l, t, u);
        } finally {
          l.refCleanup = null, l = l.alternate, l != null && (l.refCleanup = null);
        }
      else if (typeof a == "function")
        try {
          a(null);
        } catch (u) {
          fl(l, t, u);
        }
      else a.current = null;
  }
  function ko(l) {
    var t = l.type, a = l.memoizedProps, e = l.stateNode;
    try {
      l: switch (t) {
        case "button":
        case "input":
        case "select":
        case "textarea":
          a.autoFocus && e.focus();
          break l;
        case "img":
          a.src ? e.src = a.src : a.srcSet && (e.srcset = a.srcSet);
      }
    } catch (u) {
      fl(l, l.return, u);
    }
  }
  function Ec(l, t, a) {
    try {
      var e = l.stateNode;
      py(e, l.type, a, t), e[lt] = t;
    } catch (u) {
      fl(l, l.return, u);
    }
  }
  function Wo(l) {
    return l.tag === 5 || l.tag === 3 || l.tag === 26 || l.tag === 27 && _a(l.type) || l.tag === 4;
  }
  function Tc(l) {
    l: for (; ; ) {
      for (; l.sibling === null; ) {
        if (l.return === null || Wo(l.return)) return null;
        l = l.return;
      }
      for (l.sibling.return = l.return, l = l.sibling; l.tag !== 5 && l.tag !== 6 && l.tag !== 18; ) {
        if (l.tag === 27 && _a(l.type) || l.flags & 2 || l.child === null || l.tag === 4) continue l;
        l.child.return = l, l = l.child;
      }
      if (!(l.flags & 2)) return l.stateNode;
    }
  }
  function Ac(l, t, a) {
    var e = l.tag;
    if (e === 5 || e === 6)
      l = l.stateNode, t ? (a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a).insertBefore(l, t) : (t = a.nodeType === 9 ? a.body : a.nodeName === "HTML" ? a.ownerDocument.body : a, t.appendChild(l), a = a._reactRootContainer, a != null || t.onclick !== null || (t.onclick = Yt));
    else if (e !== 4 && (e === 27 && _a(l.type) && (a = l.stateNode, t = null), l = l.child, l !== null))
      for (Ac(l, t, a), l = l.sibling; l !== null; )
        Ac(l, t, a), l = l.sibling;
  }
  function rn(l, t, a) {
    var e = l.tag;
    if (e === 5 || e === 6)
      l = l.stateNode, t ? a.insertBefore(l, t) : a.appendChild(l);
    else if (e !== 4 && (e === 27 && _a(l.type) && (a = l.stateNode), l = l.child, l !== null))
      for (rn(l, t, a), l = l.sibling; l !== null; )
        rn(l, t, a), l = l.sibling;
  }
  function $o(l) {
    var t = l.stateNode, a = l.memoizedProps;
    try {
      for (var e = l.type, u = t.attributes; u.length; )
        t.removeAttributeNode(u[0]);
      Kl(t, e, a), t[Ql] = l, t[lt] = a;
    } catch (n) {
      fl(l, l.return, n);
    }
  }
  var kt = !1, Hl = !1, pc = !1, Fo = typeof WeakSet == "function" ? WeakSet : Set, Gl = null;
  function uy(l, t) {
    if (l = l.containerInfo, Jc = Cn, l = fs(l), Si(l)) {
      if ("selectionStart" in l)
        var a = {
          start: l.selectionStart,
          end: l.selectionEnd
        };
      else
        l: {
          a = (a = l.ownerDocument) && a.defaultView || window;
          var e = a.getSelection && a.getSelection();
          if (e && e.rangeCount !== 0) {
            a = e.anchorNode;
            var u = e.anchorOffset, n = e.focusNode;
            e = e.focusOffset;
            try {
              a.nodeType, n.nodeType;
            } catch {
              a = null;
              break l;
            }
            var i = 0, c = -1, f = -1, h = 0, S = 0, E = l, r = null;
            t: for (; ; ) {
              for (var g; E !== a || u !== 0 && E.nodeType !== 3 || (c = i + u), E !== n || e !== 0 && E.nodeType !== 3 || (f = i + e), E.nodeType === 3 && (i += E.nodeValue.length), (g = E.firstChild) !== null; )
                r = E, E = g;
              for (; ; ) {
                if (E === l) break t;
                if (r === a && ++h === u && (c = i), r === n && ++S === e && (f = i), (g = E.nextSibling) !== null) break;
                E = r, r = E.parentNode;
              }
              E = g;
            }
            a = c === -1 || f === -1 ? null : { start: c, end: f };
          } else a = null;
        }
      a = a || { start: 0, end: 0 };
    } else a = null;
    for (wc = { focusedElem: l, selectionRange: a }, Cn = !1, Gl = t; Gl !== null; )
      if (t = Gl, l = t.child, (t.subtreeFlags & 1028) !== 0 && l !== null)
        l.return = t, Gl = l;
      else
        for (; Gl !== null; ) {
          switch (t = Gl, n = t.alternate, l = t.flags, t.tag) {
            case 0:
              if ((l & 4) !== 0 && (l = t.updateQueue, l = l !== null ? l.events : null, l !== null))
                for (a = 0; a < l.length; a++)
                  u = l[a], u.ref.impl = u.nextImpl;
              break;
            case 11:
            case 15:
              break;
            case 1:
              if ((l & 1024) !== 0 && n !== null) {
                l = void 0, a = t, u = n.memoizedProps, n = n.memoizedState, e = a.stateNode;
                try {
                  var N = Za(
                    a.type,
                    u
                  );
                  l = e.getSnapshotBeforeUpdate(
                    N,
                    n
                  ), e.__reactInternalSnapshotBeforeUpdate = l;
                } catch (C) {
                  fl(
                    a,
                    a.return,
                    C
                  );
                }
              }
              break;
            case 3:
              if ((l & 1024) !== 0) {
                if (l = t.stateNode.containerInfo, a = l.nodeType, a === 9)
                  $c(l);
                else if (a === 1)
                  switch (l.nodeName) {
                    case "HEAD":
                    case "HTML":
                    case "BODY":
                      $c(l);
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
              if ((l & 1024) !== 0) throw Error(s(163));
          }
          if (l = t.sibling, l !== null) {
            l.return = t.return, Gl = l;
            break;
          }
          Gl = t.return;
        }
  }
  function Io(l, t, a) {
    var e = a.flags;
    switch (a.tag) {
      case 0:
      case 11:
      case 15:
        $t(l, a), e & 4 && nu(5, a);
        break;
      case 1:
        if ($t(l, a), e & 4)
          if (l = a.stateNode, t === null)
            try {
              l.componentDidMount();
            } catch (i) {
              fl(a, a.return, i);
            }
          else {
            var u = Za(
              a.type,
              t.memoizedProps
            );
            t = t.memoizedState;
            try {
              l.componentDidUpdate(
                u,
                t,
                l.__reactInternalSnapshotBeforeUpdate
              );
            } catch (i) {
              fl(
                a,
                a.return,
                i
              );
            }
          }
        e & 64 && Jo(a), e & 512 && iu(a, a.return);
        break;
      case 3:
        if ($t(l, a), e & 64 && (l = a.updateQueue, l !== null)) {
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
          } catch (i) {
            fl(a, a.return, i);
          }
        }
        break;
      case 27:
        t === null && e & 4 && $o(a);
      case 26:
      case 5:
        $t(l, a), t === null && e & 4 && ko(a), e & 512 && iu(a, a.return);
        break;
      case 12:
        $t(l, a);
        break;
      case 31:
        $t(l, a), e & 4 && td(l, a);
        break;
      case 13:
        $t(l, a), e & 4 && ad(l, a), e & 64 && (l = a.memoizedState, l !== null && (l = l.dehydrated, l !== null && (a = yy.bind(
          null,
          a
        ), Ry(l, a))));
        break;
      case 22:
        if (e = a.memoizedState !== null || kt, !e) {
          t = t !== null && t.memoizedState !== null || Hl, u = kt;
          var n = Hl;
          kt = e, (Hl = t) && !n ? Ft(
            l,
            a,
            (a.subtreeFlags & 8772) !== 0
          ) : $t(l, a), kt = u, Hl = n;
        }
        break;
      case 30:
        break;
      default:
        $t(l, a);
    }
  }
  function Po(l) {
    var t = l.alternate;
    t !== null && (l.alternate = null, Po(t)), l.child = null, l.deletions = null, l.sibling = null, l.tag === 5 && (t = l.stateNode, t !== null && ti(t)), l.stateNode = null, l.return = null, l.dependencies = null, l.memoizedProps = null, l.memoizedState = null, l.pendingProps = null, l.stateNode = null, l.updateQueue = null;
  }
  var hl = null, at = !1;
  function Wt(l, t, a) {
    for (a = a.child; a !== null; )
      ld(l, t, a), a = a.sibling;
  }
  function ld(l, t, a) {
    if (ft && typeof ft.onCommitFiberUnmount == "function")
      try {
        ft.onCommitFiberUnmount(Ne, a);
      } catch {
      }
    switch (a.tag) {
      case 26:
        Hl || xt(a, t), Wt(
          l,
          t,
          a
        ), a.memoizedState ? a.memoizedState.count-- : a.stateNode && (a = a.stateNode, a.parentNode.removeChild(a));
        break;
      case 27:
        Hl || xt(a, t);
        var e = hl, u = at;
        _a(a.type) && (hl = a.stateNode, at = !1), Wt(
          l,
          t,
          a
        ), hu(a.stateNode), hl = e, at = u;
        break;
      case 5:
        Hl || xt(a, t);
      case 6:
        if (e = hl, u = at, hl = null, Wt(
          l,
          t,
          a
        ), hl = e, at = u, hl !== null)
          if (at)
            try {
              (hl.nodeType === 9 ? hl.body : hl.nodeName === "HTML" ? hl.ownerDocument.body : hl).removeChild(a.stateNode);
            } catch (n) {
              fl(
                a,
                t,
                n
              );
            }
          else
            try {
              hl.removeChild(a.stateNode);
            } catch (n) {
              fl(
                a,
                t,
                n
              );
            }
        break;
      case 18:
        hl !== null && (at ? (l = hl, Jd(
          l.nodeType === 9 ? l.body : l.nodeName === "HTML" ? l.ownerDocument.body : l,
          a.stateNode
        ), De(l)) : Jd(hl, a.stateNode));
        break;
      case 4:
        e = hl, u = at, hl = a.stateNode.containerInfo, at = !0, Wt(
          l,
          t,
          a
        ), hl = e, at = u;
        break;
      case 0:
      case 11:
      case 14:
      case 15:
        ya(2, a, t), Hl || ya(4, a, t), Wt(
          l,
          t,
          a
        );
        break;
      case 1:
        Hl || (xt(a, t), e = a.stateNode, typeof e.componentWillUnmount == "function" && wo(
          a,
          t,
          e
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
        Hl = (e = Hl) || a.memoizedState !== null, Wt(
          l,
          t,
          a
        ), Hl = e;
        break;
      default:
        Wt(
          l,
          t,
          a
        );
    }
  }
  function td(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null))) {
      l = l.dehydrated;
      try {
        De(l);
      } catch (a) {
        fl(t, t.return, a);
      }
    }
  }
  function ad(l, t) {
    if (t.memoizedState === null && (l = t.alternate, l !== null && (l = l.memoizedState, l !== null && (l = l.dehydrated, l !== null))))
      try {
        De(l);
      } catch (a) {
        fl(t, t.return, a);
      }
  }
  function ny(l) {
    switch (l.tag) {
      case 31:
      case 13:
      case 19:
        var t = l.stateNode;
        return t === null && (t = l.stateNode = new Fo()), t;
      case 22:
        return l = l.stateNode, t = l._retryCache, t === null && (t = l._retryCache = new Fo()), t;
      default:
        throw Error(s(435, l.tag));
    }
  }
  function gn(l, t) {
    var a = ny(l);
    t.forEach(function(e) {
      if (!a.has(e)) {
        a.add(e);
        var u = my.bind(null, l, e);
        e.then(u, u);
      }
    });
  }
  function et(l, t) {
    var a = t.deletions;
    if (a !== null)
      for (var e = 0; e < a.length; e++) {
        var u = a[e], n = l, i = t, c = i;
        l: for (; c !== null; ) {
          switch (c.tag) {
            case 27:
              if (_a(c.type)) {
                hl = c.stateNode, at = !1;
                break l;
              }
              break;
            case 5:
              hl = c.stateNode, at = !1;
              break l;
            case 3:
            case 4:
              hl = c.stateNode.containerInfo, at = !0;
              break l;
          }
          c = c.return;
        }
        if (hl === null) throw Error(s(160));
        ld(n, i, u), hl = null, at = !1, n = u.alternate, n !== null && (n.return = null), u.return = null;
      }
    if (t.subtreeFlags & 13886)
      for (t = t.child; t !== null; )
        ed(t, l), t = t.sibling;
  }
  var Nt = null;
  function ed(l, t) {
    var a = l.alternate, e = l.flags;
    switch (l.tag) {
      case 0:
      case 11:
      case 14:
      case 15:
        et(t, l), ut(l), e & 4 && (ya(3, l, l.return), nu(3, l), ya(5, l, l.return));
        break;
      case 1:
        et(t, l), ut(l), e & 512 && (Hl || a === null || xt(a, a.return)), e & 64 && kt && (l = l.updateQueue, l !== null && (e = l.callbacks, e !== null && (a = l.shared.hiddenCallbacks, l.shared.hiddenCallbacks = a === null ? e : a.concat(e))));
        break;
      case 26:
        var u = Nt;
        if (et(t, l), ut(l), e & 512 && (Hl || a === null || xt(a, a.return)), e & 4) {
          var n = a !== null ? a.memoizedState : null;
          if (e = l.memoizedState, a === null)
            if (e === null)
              if (l.stateNode === null) {
                l: {
                  e = l.type, a = l.memoizedProps, u = u.ownerDocument || u;
                  t: switch (e) {
                    case "title":
                      n = u.getElementsByTagName("title")[0], (!n || n[Re] || n[Ql] || n.namespaceURI === "http://www.w3.org/2000/svg" || n.hasAttribute("itemprop")) && (n = u.createElement(e), u.head.insertBefore(
                        n,
                        u.querySelector("head > title")
                      )), Kl(n, e, a), n[Ql] = l, Yl(n), e = n;
                      break l;
                    case "link":
                      var i = ev(
                        "link",
                        "href",
                        u
                      ).get(e + (a.href || ""));
                      if (i) {
                        for (var c = 0; c < i.length; c++)
                          if (n = i[c], n.getAttribute("href") === (a.href == null || a.href === "" ? null : a.href) && n.getAttribute("rel") === (a.rel == null ? null : a.rel) && n.getAttribute("title") === (a.title == null ? null : a.title) && n.getAttribute("crossorigin") === (a.crossOrigin == null ? null : a.crossOrigin)) {
                            i.splice(c, 1);
                            break t;
                          }
                      }
                      n = u.createElement(e), Kl(n, e, a), u.head.appendChild(n);
                      break;
                    case "meta":
                      if (i = ev(
                        "meta",
                        "content",
                        u
                      ).get(e + (a.content || ""))) {
                        for (c = 0; c < i.length; c++)
                          if (n = i[c], n.getAttribute("content") === (a.content == null ? null : "" + a.content) && n.getAttribute("name") === (a.name == null ? null : a.name) && n.getAttribute("property") === (a.property == null ? null : a.property) && n.getAttribute("http-equiv") === (a.httpEquiv == null ? null : a.httpEquiv) && n.getAttribute("charset") === (a.charSet == null ? null : a.charSet)) {
                            i.splice(c, 1);
                            break t;
                          }
                      }
                      n = u.createElement(e), Kl(n, e, a), u.head.appendChild(n);
                      break;
                    default:
                      throw Error(s(468, e));
                  }
                  n[Ql] = l, Yl(n), e = n;
                }
                l.stateNode = e;
              } else
                uv(
                  u,
                  l.type,
                  l.stateNode
                );
            else
              l.stateNode = av(
                u,
                e,
                l.memoizedProps
              );
          else
            n !== e ? (n === null ? a.stateNode !== null && (a = a.stateNode, a.parentNode.removeChild(a)) : n.count--, e === null ? uv(
              u,
              l.type,
              l.stateNode
            ) : av(
              u,
              e,
              l.memoizedProps
            )) : e === null && l.stateNode !== null && Ec(
              l,
              l.memoizedProps,
              a.memoizedProps
            );
        }
        break;
      case 27:
        et(t, l), ut(l), e & 512 && (Hl || a === null || xt(a, a.return)), a !== null && e & 4 && Ec(
          l,
          l.memoizedProps,
          a.memoizedProps
        );
        break;
      case 5:
        if (et(t, l), ut(l), e & 512 && (Hl || a === null || xt(a, a.return)), l.flags & 32) {
          u = l.stateNode;
          try {
            Fa(u, "");
          } catch (N) {
            fl(l, l.return, N);
          }
        }
        e & 4 && l.stateNode != null && (u = l.memoizedProps, Ec(
          l,
          u,
          a !== null ? a.memoizedProps : u
        )), e & 1024 && (pc = !0);
        break;
      case 6:
        if (et(t, l), ut(l), e & 4) {
          if (l.stateNode === null)
            throw Error(s(162));
          e = l.memoizedProps, a = l.stateNode;
          try {
            a.nodeValue = e;
          } catch (N) {
            fl(l, l.return, N);
          }
        }
        break;
      case 3:
        if (Rn = null, u = Nt, Nt = jn(t.containerInfo), et(t, l), Nt = u, ut(l), e & 4 && a !== null && a.memoizedState.isDehydrated)
          try {
            De(t.containerInfo);
          } catch (N) {
            fl(l, l.return, N);
          }
        pc && (pc = !1, ud(l));
        break;
      case 4:
        e = Nt, Nt = jn(
          l.stateNode.containerInfo
        ), et(t, l), ut(l), Nt = e;
        break;
      case 12:
        et(t, l), ut(l);
        break;
      case 31:
        et(t, l), ut(l), e & 4 && (e = l.updateQueue, e !== null && (l.updateQueue = null, gn(l, e)));
        break;
      case 13:
        et(t, l), ut(l), l.child.flags & 8192 && l.memoizedState !== null != (a !== null && a.memoizedState !== null) && (bn = ct()), e & 4 && (e = l.updateQueue, e !== null && (l.updateQueue = null, gn(l, e)));
        break;
      case 22:
        u = l.memoizedState !== null;
        var f = a !== null && a.memoizedState !== null, h = kt, S = Hl;
        if (kt = h || u, Hl = S || f, et(t, l), Hl = S, kt = h, ut(l), e & 8192)
          l: for (t = l.stateNode, t._visibility = u ? t._visibility & -2 : t._visibility | 1, u && (a === null || f || kt || Hl || La(l)), a = null, t = l; ; ) {
            if (t.tag === 5 || t.tag === 26) {
              if (a === null) {
                f = a = t;
                try {
                  if (n = f.stateNode, u)
                    i = n.style, typeof i.setProperty == "function" ? i.setProperty("display", "none", "important") : i.display = "none";
                  else {
                    c = f.stateNode;
                    var E = f.memoizedProps.style, r = E != null && E.hasOwnProperty("display") ? E.display : null;
                    c.style.display = r == null || typeof r == "boolean" ? "" : ("" + r).trim();
                  }
                } catch (N) {
                  fl(f, f.return, N);
                }
              }
            } else if (t.tag === 6) {
              if (a === null) {
                f = t;
                try {
                  f.stateNode.nodeValue = u ? "" : f.memoizedProps;
                } catch (N) {
                  fl(f, f.return, N);
                }
              }
            } else if (t.tag === 18) {
              if (a === null) {
                f = t;
                try {
                  var g = f.stateNode;
                  u ? wd(g, !0) : wd(f.stateNode, !1);
                } catch (N) {
                  fl(f, f.return, N);
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
        e & 4 && (e = l.updateQueue, e !== null && (a = e.retryQueue, a !== null && (e.retryQueue = null, gn(l, a))));
        break;
      case 19:
        et(t, l), ut(l), e & 4 && (e = l.updateQueue, e !== null && (l.updateQueue = null, gn(l, e)));
        break;
      case 30:
        break;
      case 21:
        break;
      default:
        et(t, l), ut(l);
    }
  }
  function ut(l) {
    var t = l.flags;
    if (t & 2) {
      try {
        for (var a, e = l.return; e !== null; ) {
          if (Wo(e)) {
            a = e;
            break;
          }
          e = e.return;
        }
        if (a == null) throw Error(s(160));
        switch (a.tag) {
          case 27:
            var u = a.stateNode, n = Tc(l);
            rn(l, n, u);
            break;
          case 5:
            var i = a.stateNode;
            a.flags & 32 && (Fa(i, ""), a.flags &= -33);
            var c = Tc(l);
            rn(l, c, i);
            break;
          case 3:
          case 4:
            var f = a.stateNode.containerInfo, h = Tc(l);
            Ac(
              l,
              h,
              f
            );
            break;
          default:
            throw Error(s(161));
        }
      } catch (S) {
        fl(l, l.return, S);
      }
      l.flags &= -3;
    }
    t & 4096 && (l.flags &= -4097);
  }
  function ud(l) {
    if (l.subtreeFlags & 1024)
      for (l = l.child; l !== null; ) {
        var t = l;
        ud(t), t.tag === 5 && t.flags & 1024 && t.stateNode.reset(), l = l.sibling;
      }
  }
  function $t(l, t) {
    if (t.subtreeFlags & 8772)
      for (t = t.child; t !== null; )
        Io(l, t.alternate, t), t = t.sibling;
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
          xt(t, t.return);
          var a = t.stateNode;
          typeof a.componentWillUnmount == "function" && wo(
            t,
            t.return,
            a
          ), La(t);
          break;
        case 27:
          hu(t.stateNode);
        case 26:
        case 5:
          xt(t, t.return), La(t);
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
      var e = t.alternate, u = l, n = t, i = n.flags;
      switch (n.tag) {
        case 0:
        case 11:
        case 15:
          Ft(
            u,
            n,
            a
          ), nu(4, n);
          break;
        case 1:
          if (Ft(
            u,
            n,
            a
          ), e = n, u = e.stateNode, typeof u.componentDidMount == "function")
            try {
              u.componentDidMount();
            } catch (h) {
              fl(e, e.return, h);
            }
          if (e = n, u = e.updateQueue, u !== null) {
            var c = e.stateNode;
            try {
              var f = u.shared.hiddenCallbacks;
              if (f !== null)
                for (u.shared.hiddenCallbacks = null, u = 0; u < f.length; u++)
                  xs(f[u], c);
            } catch (h) {
              fl(e, e.return, h);
            }
          }
          a && i & 64 && Jo(n), iu(n, n.return);
          break;
        case 27:
          $o(n);
        case 26:
        case 5:
          Ft(
            u,
            n,
            a
          ), a && e === null && i & 4 && ko(n), iu(n, n.return);
          break;
        case 12:
          Ft(
            u,
            n,
            a
          );
          break;
        case 31:
          Ft(
            u,
            n,
            a
          ), a && i & 4 && td(u, n);
          break;
        case 13:
          Ft(
            u,
            n,
            a
          ), a && i & 4 && ad(u, n);
          break;
        case 22:
          n.memoizedState === null && Ft(
            u,
            n,
            a
          ), iu(n, n.return);
          break;
        case 30:
          break;
        default:
          Ft(
            u,
            n,
            a
          );
      }
      t = t.sibling;
    }
  }
  function Mc(l, t) {
    var a = null;
    l !== null && l.memoizedState !== null && l.memoizedState.cachePool !== null && (a = l.memoizedState.cachePool.pool), l = null, t.memoizedState !== null && t.memoizedState.cachePool !== null && (l = t.memoizedState.cachePool.pool), l !== a && (l != null && l.refCount++, a != null && Je(a));
  }
  function Oc(l, t) {
    l = null, t.alternate !== null && (l = t.alternate.memoizedState.cache), t = t.memoizedState.cache, t !== l && (t.refCount++, l != null && Je(l));
  }
  function jt(l, t, a, e) {
    if (t.subtreeFlags & 10256)
      for (t = t.child; t !== null; )
        nd(
          l,
          t,
          a,
          e
        ), t = t.sibling;
  }
  function nd(l, t, a, e) {
    var u = t.flags;
    switch (t.tag) {
      case 0:
      case 11:
      case 15:
        jt(
          l,
          t,
          a,
          e
        ), u & 2048 && nu(9, t);
        break;
      case 1:
        jt(
          l,
          t,
          a,
          e
        );
        break;
      case 3:
        jt(
          l,
          t,
          a,
          e
        ), u & 2048 && (l = null, t.alternate !== null && (l = t.alternate.memoizedState.cache), t = t.memoizedState.cache, t !== l && (t.refCount++, l != null && Je(l)));
        break;
      case 12:
        if (u & 2048) {
          jt(
            l,
            t,
            a,
            e
          ), l = t.stateNode;
          try {
            var n = t.memoizedProps, i = n.id, c = n.onPostCommit;
            typeof c == "function" && c(
              i,
              t.alternate === null ? "mount" : "update",
              l.passiveEffectDuration,
              -0
            );
          } catch (f) {
            fl(t, t.return, f);
          }
        } else
          jt(
            l,
            t,
            a,
            e
          );
        break;
      case 31:
        jt(
          l,
          t,
          a,
          e
        );
        break;
      case 13:
        jt(
          l,
          t,
          a,
          e
        );
        break;
      case 23:
        break;
      case 22:
        n = t.stateNode, i = t.alternate, t.memoizedState !== null ? n._visibility & 2 ? jt(
          l,
          t,
          a,
          e
        ) : cu(l, t) : n._visibility & 2 ? jt(
          l,
          t,
          a,
          e
        ) : (n._visibility |= 2, ge(
          l,
          t,
          a,
          e,
          (t.subtreeFlags & 10256) !== 0 || !1
        )), u & 2048 && Mc(i, t);
        break;
      case 24:
        jt(
          l,
          t,
          a,
          e
        ), u & 2048 && Oc(t.alternate, t);
        break;
      default:
        jt(
          l,
          t,
          a,
          e
        );
    }
  }
  function ge(l, t, a, e, u) {
    for (u = u && ((t.subtreeFlags & 10256) !== 0 || !1), t = t.child; t !== null; ) {
      var n = l, i = t, c = a, f = e, h = i.flags;
      switch (i.tag) {
        case 0:
        case 11:
        case 15:
          ge(
            n,
            i,
            c,
            f,
            u
          ), nu(8, i);
          break;
        case 23:
          break;
        case 22:
          var S = i.stateNode;
          i.memoizedState !== null ? S._visibility & 2 ? ge(
            n,
            i,
            c,
            f,
            u
          ) : cu(
            n,
            i
          ) : (S._visibility |= 2, ge(
            n,
            i,
            c,
            f,
            u
          )), u && h & 2048 && Mc(
            i.alternate,
            i
          );
          break;
        case 24:
          ge(
            n,
            i,
            c,
            f,
            u
          ), u && h & 2048 && Oc(i.alternate, i);
          break;
        default:
          ge(
            n,
            i,
            c,
            f,
            u
          );
      }
      t = t.sibling;
    }
  }
  function cu(l, t) {
    if (t.subtreeFlags & 10256)
      for (t = t.child; t !== null; ) {
        var a = l, e = t, u = e.flags;
        switch (e.tag) {
          case 22:
            cu(a, e), u & 2048 && Mc(
              e.alternate,
              e
            );
            break;
          case 24:
            cu(a, e), u & 2048 && Oc(e.alternate, e);
            break;
          default:
            cu(a, e);
        }
        t = t.sibling;
      }
  }
  var fu = 8192;
  function Se(l, t, a) {
    if (l.subtreeFlags & fu)
      for (l = l.child; l !== null; )
        id(
          l,
          t,
          a
        ), l = l.sibling;
  }
  function id(l, t, a) {
    switch (l.tag) {
      case 26:
        Se(
          l,
          t,
          a
        ), l.flags & fu && l.memoizedState !== null && Ky(
          a,
          Nt,
          l.memoizedState,
          l.memoizedProps
        );
        break;
      case 5:
        Se(
          l,
          t,
          a
        );
        break;
      case 3:
      case 4:
        var e = Nt;
        Nt = jn(l.stateNode.containerInfo), Se(
          l,
          t,
          a
        ), Nt = e;
        break;
      case 22:
        l.memoizedState === null && (e = l.alternate, e !== null && e.memoizedState !== null ? (e = fu, fu = 16777216, Se(
          l,
          t,
          a
        ), fu = e) : Se(
          l,
          t,
          a
        ));
        break;
      default:
        Se(
          l,
          t,
          a
        );
    }
  }
  function cd(l) {
    var t = l.alternate;
    if (t !== null && (l = t.child, l !== null)) {
      t.child = null;
      do
        t = l.sibling, l.sibling = null, l = t;
      while (l !== null);
    }
  }
  function su(l) {
    var t = l.deletions;
    if ((l.flags & 16) !== 0) {
      if (t !== null)
        for (var a = 0; a < t.length; a++) {
          var e = t[a];
          Gl = e, sd(
            e,
            l
          );
        }
      cd(l);
    }
    if (l.subtreeFlags & 10256)
      for (l = l.child; l !== null; )
        fd(l), l = l.sibling;
  }
  function fd(l) {
    switch (l.tag) {
      case 0:
      case 11:
      case 15:
        su(l), l.flags & 2048 && ya(9, l, l.return);
        break;
      case 3:
        su(l);
        break;
      case 12:
        su(l);
        break;
      case 22:
        var t = l.stateNode;
        l.memoizedState !== null && t._visibility & 2 && (l.return === null || l.return.tag !== 13) ? (t._visibility &= -3, Sn(l)) : su(l);
        break;
      default:
        su(l);
    }
  }
  function Sn(l) {
    var t = l.deletions;
    if ((l.flags & 16) !== 0) {
      if (t !== null)
        for (var a = 0; a < t.length; a++) {
          var e = t[a];
          Gl = e, sd(
            e,
            l
          );
        }
      cd(l);
    }
    for (l = l.child; l !== null; ) {
      switch (t = l, t.tag) {
        case 0:
        case 11:
        case 15:
          ya(8, t, t.return), Sn(t);
          break;
        case 22:
          a = t.stateNode, a._visibility & 2 && (a._visibility &= -3, Sn(t));
          break;
        default:
          Sn(t);
      }
      l = l.sibling;
    }
  }
  function sd(l, t) {
    for (; Gl !== null; ) {
      var a = Gl;
      switch (a.tag) {
        case 0:
        case 11:
        case 15:
          ya(8, a, t);
          break;
        case 23:
        case 22:
          if (a.memoizedState !== null && a.memoizedState.cachePool !== null) {
            var e = a.memoizedState.cachePool.pool;
            e != null && e.refCount++;
          }
          break;
        case 24:
          Je(a.memoizedState.cache);
      }
      if (e = a.child, e !== null) e.return = a, Gl = e;
      else
        l: for (a = l; Gl !== null; ) {
          e = Gl;
          var u = e.sibling, n = e.return;
          if (Po(e), e === a) {
            Gl = null;
            break l;
          }
          if (u !== null) {
            u.return = n, Gl = u;
            break l;
          }
          Gl = n;
        }
    }
  }
  var iy = {
    getCacheForType: function(l) {
      var t = Ll(Ul), a = t.data.get(l);
      return a === void 0 && (a = l(), t.data.set(l, a)), a;
    },
    cacheSignal: function() {
      return Ll(Ul).controller.signal;
    }
  }, cy = typeof WeakMap == "function" ? WeakMap : Map, ul = 0, vl = null, K = null, w = 0, cl = 0, mt = null, ma = !1, be = !1, Dc = !1, It = 0, _l = 0, ha = 0, Va = 0, Uc = 0, ht = 0, _e = 0, ou = null, nt = null, Nc = !1, bn = 0, od = 0, _n = 1 / 0, zn = null, ra = null, Bl = 0, ga = null, ze = null, Pt = 0, jc = 0, Hc = null, dd = null, du = 0, Rc = null;
  function rt() {
    return (ul & 2) !== 0 && w !== 0 ? w & -w : b.T !== null ? Gc() : Mf();
  }
  function vd() {
    if (ht === 0)
      if ((w & 536870912) === 0 || $) {
        var l = Du;
        Du <<= 1, (Du & 3932160) === 0 && (Du = 262144), ht = l;
      } else ht = 536870912;
    return l = vt.current, l !== null && (l.flags |= 32), ht;
  }
  function it(l, t, a) {
    (l === vl && (cl === 2 || cl === 9) || l.cancelPendingCommit !== null) && (Ee(l, 0), Sa(
      l,
      w,
      ht,
      !1
    )), He(l, a), ((ul & 2) === 0 || l !== vl) && (l === vl && ((ul & 2) === 0 && (Va |= a), _l === 4 && Sa(
      l,
      w,
      ht,
      !1
    )), qt(l));
  }
  function yd(l, t, a) {
    if ((ul & 6) !== 0) throw Error(s(327));
    var e = !a && (t & 127) === 0 && (t & l.expiredLanes) === 0 || je(l, t), u = e ? oy(l, t) : qc(l, t, !0), n = e;
    do {
      if (u === 0) {
        be && !e && Sa(l, t, 0, !1);
        break;
      } else {
        if (a = l.current.alternate, n && !fy(a)) {
          u = qc(l, t, !1), n = !1;
          continue;
        }
        if (u === 2) {
          if (n = t, l.errorRecoveryDisabledLanes & n)
            var i = 0;
          else
            i = l.pendingLanes & -536870913, i = i !== 0 ? i : i & 536870912 ? 536870912 : 0;
          if (i !== 0) {
            t = i;
            l: {
              var c = l;
              u = ou;
              var f = c.current.memoizedState.isDehydrated;
              if (f && (Ee(c, i).flags |= 256), i = qc(
                c,
                i,
                !1
              ), i !== 2) {
                if (Dc && !f) {
                  c.errorRecoveryDisabledLanes |= n, Va |= n, u = 4;
                  break l;
                }
                n = nt, nt = u, n !== null && (nt === null ? nt = n : nt.push.apply(
                  nt,
                  n
                ));
              }
              u = i;
            }
            if (n = !1, u !== 2) continue;
          }
        }
        if (u === 1) {
          Ee(l, 0), Sa(l, t, 0, !0);
          break;
        }
        l: {
          switch (e = l, n = u, n) {
            case 0:
            case 1:
              throw Error(s(345));
            case 4:
              if ((t & 4194048) !== t) break;
            case 6:
              Sa(
                e,
                t,
                ht,
                !ma
              );
              break l;
            case 2:
              nt = null;
              break;
            case 3:
            case 5:
              break;
            default:
              throw Error(s(329));
          }
          if ((t & 62914560) === t && (u = bn + 300 - ct(), 10 < u)) {
            if (Sa(
              e,
              t,
              ht,
              !ma
            ), Nu(e, 0, !0) !== 0) break l;
            Pt = t, e.timeoutHandle = Vd(
              md.bind(
                null,
                e,
                a,
                nt,
                zn,
                Nc,
                t,
                ht,
                Va,
                _e,
                ma,
                n,
                "Throttled",
                -0,
                0
              ),
              u
            );
            break l;
          }
          md(
            e,
            a,
            nt,
            zn,
            Nc,
            t,
            ht,
            Va,
            _e,
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
    qt(l);
  }
  function md(l, t, a, e, u, n, i, c, f, h, S, E, r, g) {
    if (l.timeoutHandle = -1, E = t.subtreeFlags, E & 8192 || (E & 16785408) === 16785408) {
      E = {
        stylesheets: null,
        count: 0,
        imgCount: 0,
        imgBytes: 0,
        suspenseyImages: [],
        waitingForImages: !0,
        waitingForViewTransition: !1,
        unsuspend: Yt
      }, id(
        t,
        n,
        E
      );
      var N = (n & 62914560) === n ? bn - ct() : (n & 4194048) === n ? od - ct() : 0;
      if (N = Jy(
        E,
        N
      ), N !== null) {
        Pt = n, l.cancelPendingCommit = N(
          Ed.bind(
            null,
            l,
            t,
            n,
            a,
            e,
            u,
            i,
            c,
            f,
            S,
            E,
            null,
            r,
            g
          )
        ), Sa(l, n, i, !h);
        return;
      }
    }
    Ed(
      l,
      t,
      n,
      a,
      e,
      u,
      i,
      c,
      f
    );
  }
  function fy(l) {
    for (var t = l; ; ) {
      var a = t.tag;
      if ((a === 0 || a === 11 || a === 15) && t.flags & 16384 && (a = t.updateQueue, a !== null && (a = a.stores, a !== null)))
        for (var e = 0; e < a.length; e++) {
          var u = a[e], n = u.getSnapshot;
          u = u.value;
          try {
            if (!ot(n(), u)) return !1;
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
  function Sa(l, t, a, e) {
    t &= ~Uc, t &= ~Va, l.suspendedLanes |= t, l.pingedLanes &= ~t, e && (l.warmLanes |= t), e = l.expirationTimes;
    for (var u = t; 0 < u; ) {
      var n = 31 - st(u), i = 1 << n;
      e[n] = -1, u &= ~i;
    }
    a !== 0 && Tf(l, a, t);
  }
  function En() {
    return (ul & 6) === 0 ? (vu(0), !1) : !0;
  }
  function xc() {
    if (K !== null) {
      if (cl === 0)
        var l = K.return;
      else
        l = K, Zt = qa = null, $i(l), ve = null, ke = 0, l = K;
      for (; l !== null; )
        Ko(l.alternate, l), l = l.return;
      K = null;
    }
  }
  function Ee(l, t) {
    var a = l.timeoutHandle;
    a !== -1 && (l.timeoutHandle = -1, Dy(a)), a = l.cancelPendingCommit, a !== null && (l.cancelPendingCommit = null, a()), Pt = 0, xc(), vl = l, K = a = Xt(l.current, null), w = t, cl = 0, mt = null, ma = !1, be = je(l, t), Dc = !1, _e = ht = Uc = Va = ha = _l = 0, nt = ou = null, Nc = !1, (t & 8) !== 0 && (t |= t & 32);
    var e = l.entangledLanes;
    if (e !== 0)
      for (l = l.entanglements, e &= t; 0 < e; ) {
        var u = 31 - st(e), n = 1 << u;
        t |= l[u], e &= ~n;
      }
    return It = t, Zu(), a;
  }
  function hd(l, t) {
    Q = null, b.H = au, t === de || t === $u ? (t = Ns(), cl = 3) : t === Yi ? (t = Ns(), cl = 4) : cl = t === vc ? 8 : t !== null && typeof t == "object" && typeof t.then == "function" ? 6 : 1, mt = t, K === null && (_l = 1, dn(
      l,
      Et(t, l.current)
    ));
  }
  function rd() {
    var l = vt.current;
    return l === null ? !0 : (w & 4194048) === w ? Mt === null : (w & 62914560) === w || (w & 536870912) !== 0 ? l === Mt : !1;
  }
  function gd() {
    var l = b.H;
    return b.H = au, l === null ? au : l;
  }
  function Sd() {
    var l = b.A;
    return b.A = iy, l;
  }
  function Tn() {
    _l = 4, ma || (w & 4194048) !== w && vt.current !== null || (be = !0), (ha & 134217727) === 0 && (Va & 134217727) === 0 || vl === null || Sa(
      vl,
      w,
      ht,
      !1
    );
  }
  function qc(l, t, a) {
    var e = ul;
    ul |= 2;
    var u = gd(), n = Sd();
    (vl !== l || w !== t) && (zn = null, Ee(l, t)), t = !1;
    var i = _l;
    l: do
      try {
        if (cl !== 0 && K !== null) {
          var c = K, f = mt;
          switch (cl) {
            case 8:
              xc(), i = 6;
              break l;
            case 3:
            case 2:
            case 9:
            case 6:
              vt.current === null && (t = !0);
              var h = cl;
              if (cl = 0, mt = null, Te(l, c, f, h), a && be) {
                i = 0;
                break l;
              }
              break;
            default:
              h = cl, cl = 0, mt = null, Te(l, c, f, h);
          }
        }
        sy(), i = _l;
        break;
      } catch (S) {
        hd(l, S);
      }
    while (!0);
    return t && l.shellSuspendCounter++, Zt = qa = null, ul = e, b.H = u, b.A = n, K === null && (vl = null, w = 0, Zu()), i;
  }
  function sy() {
    for (; K !== null; ) bd(K);
  }
  function oy(l, t) {
    var a = ul;
    ul |= 2;
    var e = gd(), u = Sd();
    vl !== l || w !== t ? (zn = null, _n = ct() + 500, Ee(l, t)) : be = je(
      l,
      t
    );
    l: do
      try {
        if (cl !== 0 && K !== null) {
          t = K;
          var n = mt;
          t: switch (cl) {
            case 1:
              cl = 0, mt = null, Te(l, t, n, 1);
              break;
            case 2:
            case 9:
              if (Ds(n)) {
                cl = 0, mt = null, _d(t);
                break;
              }
              t = function() {
                cl !== 2 && cl !== 9 || vl !== l || (cl = 7), qt(l);
              }, n.then(t, t);
              break l;
            case 3:
              cl = 7;
              break l;
            case 4:
              cl = 5;
              break l;
            case 7:
              Ds(n) ? (cl = 0, mt = null, _d(t)) : (cl = 0, mt = null, Te(l, t, n, 7));
              break;
            case 5:
              var i = null;
              switch (K.tag) {
                case 26:
                  i = K.memoizedState;
                case 5:
                case 27:
                  var c = K;
                  if (i ? nv(i) : c.stateNode.complete) {
                    cl = 0, mt = null;
                    var f = c.sibling;
                    if (f !== null) K = f;
                    else {
                      var h = c.return;
                      h !== null ? (K = h, An(h)) : K = null;
                    }
                    break t;
                  }
              }
              cl = 0, mt = null, Te(l, t, n, 5);
              break;
            case 6:
              cl = 0, mt = null, Te(l, t, n, 6);
              break;
            case 8:
              xc(), _l = 6;
              break l;
            default:
              throw Error(s(462));
          }
        }
        dy();
        break;
      } catch (S) {
        hd(l, S);
      }
    while (!0);
    return Zt = qa = null, b.H = e, b.A = u, ul = a, K !== null ? 0 : (vl = null, w = 0, Zu(), _l);
  }
  function dy() {
    for (; K !== null && !xv(); )
      bd(K);
  }
  function bd(l) {
    var t = Lo(l.alternate, l, It);
    l.memoizedProps = l.pendingProps, t === null ? An(l) : K = t;
  }
  function _d(l) {
    var t = l, a = t.alternate;
    switch (t.tag) {
      case 15:
      case 0:
        t = Co(
          a,
          t,
          t.pendingProps,
          t.type,
          void 0,
          w
        );
        break;
      case 11:
        t = Co(
          a,
          t,
          t.pendingProps,
          t.type.render,
          t.ref,
          w
        );
        break;
      case 5:
        $i(t);
      default:
        Ko(a, t), t = K = gs(t, It), t = Lo(a, t, It);
    }
    l.memoizedProps = l.pendingProps, t === null ? An(l) : K = t;
  }
  function Te(l, t, a, e) {
    Zt = qa = null, $i(t), ve = null, ke = 0;
    var u = t.return;
    try {
      if (P0(
        l,
        u,
        t,
        a,
        w
      )) {
        _l = 1, dn(
          l,
          Et(a, l.current)
        ), K = null;
        return;
      }
    } catch (n) {
      if (u !== null) throw K = u, n;
      _l = 1, dn(
        l,
        Et(a, l.current)
      ), K = null;
      return;
    }
    t.flags & 32768 ? ($ || e === 1 ? l = !0 : be || (w & 536870912) !== 0 ? l = !1 : (ma = l = !0, (e === 2 || e === 9 || e === 3 || e === 6) && (e = vt.current, e !== null && e.tag === 13 && (e.flags |= 16384))), zd(t, l)) : An(t);
  }
  function An(l) {
    var t = l;
    do {
      if ((t.flags & 32768) !== 0) {
        zd(
          t,
          ma
        );
        return;
      }
      l = t.return;
      var a = ay(
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
    _l === 0 && (_l = 5);
  }
  function zd(l, t) {
    do {
      var a = ey(l.alternate, l);
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
    _l = 6, K = null;
  }
  function Ed(l, t, a, e, u, n, i, c, f) {
    l.cancelPendingCommit = null;
    do
      pn();
    while (Bl !== 0);
    if ((ul & 6) !== 0) throw Error(s(327));
    if (t !== null) {
      if (t === l.current) throw Error(s(177));
      if (n = t.lanes | t.childLanes, n |= Ti, Vv(
        l,
        a,
        n,
        i,
        c,
        f
      ), l === vl && (K = vl = null, w = 0), ze = t, ga = l, Pt = a, jc = n, Hc = u, dd = e, (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? (l.callbackNode = null, l.callbackPriority = 0, hy(Mu, function() {
        return Od(), null;
      })) : (l.callbackNode = null, l.callbackPriority = 0), e = (t.flags & 13878) !== 0, (t.subtreeFlags & 13878) !== 0 || e) {
        e = b.T, b.T = null, u = _.p, _.p = 2, i = ul, ul |= 4;
        try {
          uy(l, t, a);
        } finally {
          ul = i, _.p = u, b.T = e;
        }
      }
      Bl = 1, Td(), Ad(), pd();
    }
  }
  function Td() {
    if (Bl === 1) {
      Bl = 0;
      var l = ga, t = ze, a = (t.flags & 13878) !== 0;
      if ((t.subtreeFlags & 13878) !== 0 || a) {
        a = b.T, b.T = null;
        var e = _.p;
        _.p = 2;
        var u = ul;
        ul |= 4;
        try {
          ed(t, l);
          var n = wc, i = fs(l.containerInfo), c = n.focusedElem, f = n.selectionRange;
          if (i !== c && c && c.ownerDocument && cs(
            c.ownerDocument.documentElement,
            c
          )) {
            if (f !== null && Si(c)) {
              var h = f.start, S = f.end;
              if (S === void 0 && (S = h), "selectionStart" in c)
                c.selectionStart = h, c.selectionEnd = Math.min(
                  S,
                  c.value.length
                );
              else {
                var E = c.ownerDocument || document, r = E && E.defaultView || window;
                if (r.getSelection) {
                  var g = r.getSelection(), N = c.textContent.length, C = Math.min(f.start, N), dl = f.end === void 0 ? C : Math.min(f.end, N);
                  !g.extend && C > dl && (i = dl, dl = C, C = i);
                  var y = is(
                    c,
                    C
                  ), o = is(
                    c,
                    dl
                  );
                  if (y && o && (g.rangeCount !== 1 || g.anchorNode !== y.node || g.anchorOffset !== y.offset || g.focusNode !== o.node || g.focusOffset !== o.offset)) {
                    var m = E.createRange();
                    m.setStart(y.node, y.offset), g.removeAllRanges(), C > dl ? (g.addRange(m), g.extend(o.node, o.offset)) : (m.setEnd(o.node, o.offset), g.addRange(m));
                  }
                }
              }
            }
            for (E = [], g = c; g = g.parentNode; )
              g.nodeType === 1 && E.push({
                element: g,
                left: g.scrollLeft,
                top: g.scrollTop
              });
            for (typeof c.focus == "function" && c.focus(), c = 0; c < E.length; c++) {
              var z = E[c];
              z.element.scrollLeft = z.left, z.element.scrollTop = z.top;
            }
          }
          Cn = !!Jc, wc = Jc = null;
        } finally {
          ul = u, _.p = e, b.T = a;
        }
      }
      l.current = t, Bl = 2;
    }
  }
  function Ad() {
    if (Bl === 2) {
      Bl = 0;
      var l = ga, t = ze, a = (t.flags & 8772) !== 0;
      if ((t.subtreeFlags & 8772) !== 0 || a) {
        a = b.T, b.T = null;
        var e = _.p;
        _.p = 2;
        var u = ul;
        ul |= 4;
        try {
          Io(l, t.alternate, t);
        } finally {
          ul = u, _.p = e, b.T = a;
        }
      }
      Bl = 3;
    }
  }
  function pd() {
    if (Bl === 4 || Bl === 3) {
      Bl = 0, qv();
      var l = ga, t = ze, a = Pt, e = dd;
      (t.subtreeFlags & 10256) !== 0 || (t.flags & 10256) !== 0 ? Bl = 5 : (Bl = 0, ze = ga = null, Md(l, l.pendingLanes));
      var u = l.pendingLanes;
      if (u === 0 && (ra = null), Pn(a), t = t.stateNode, ft && typeof ft.onCommitFiberRoot == "function")
        try {
          ft.onCommitFiberRoot(
            Ne,
            t,
            void 0,
            (t.current.flags & 128) === 128
          );
        } catch {
        }
      if (e !== null) {
        t = b.T, u = _.p, _.p = 2, b.T = null;
        try {
          for (var n = l.onRecoverableError, i = 0; i < e.length; i++) {
            var c = e[i];
            n(c.value, {
              componentStack: c.stack
            });
          }
        } finally {
          b.T = t, _.p = u;
        }
      }
      (Pt & 3) !== 0 && pn(), qt(l), u = l.pendingLanes, (a & 261930) !== 0 && (u & 42) !== 0 ? l === Rc ? du++ : (du = 0, Rc = l) : du = 0, vu(0);
    }
  }
  function Md(l, t) {
    (l.pooledCacheLanes &= t) === 0 && (t = l.pooledCache, t != null && (l.pooledCache = null, Je(t)));
  }
  function pn() {
    return Td(), Ad(), pd(), Od();
  }
  function Od() {
    if (Bl !== 5) return !1;
    var l = ga, t = jc;
    jc = 0;
    var a = Pn(Pt), e = b.T, u = _.p;
    try {
      _.p = 32 > a ? 32 : a, b.T = null, a = Hc, Hc = null;
      var n = ga, i = Pt;
      if (Bl = 0, ze = ga = null, Pt = 0, (ul & 6) !== 0) throw Error(s(331));
      var c = ul;
      if (ul |= 4, fd(n.current), nd(
        n,
        n.current,
        i,
        a
      ), ul = c, vu(0, !1), ft && typeof ft.onPostCommitFiberRoot == "function")
        try {
          ft.onPostCommitFiberRoot(Ne, n);
        } catch {
        }
      return !0;
    } finally {
      _.p = u, b.T = e, Md(l, t);
    }
  }
  function Dd(l, t, a) {
    t = Et(a, t), t = dc(l.stateNode, t, 2), l = oa(l, t, 2), l !== null && (He(l, 2), qt(l));
  }
  function fl(l, t, a) {
    if (l.tag === 3)
      Dd(l, l, a);
    else
      for (; t !== null; ) {
        if (t.tag === 3) {
          Dd(
            t,
            l,
            a
          );
          break;
        } else if (t.tag === 1) {
          var e = t.stateNode;
          if (typeof t.type.getDerivedStateFromError == "function" || typeof e.componentDidCatch == "function" && (ra === null || !ra.has(e))) {
            l = Et(a, l), a = Uo(2), e = oa(t, a, 2), e !== null && (No(
              a,
              e,
              t,
              l
            ), He(e, 2), qt(e));
            break;
          }
        }
        t = t.return;
      }
  }
  function Bc(l, t, a) {
    var e = l.pingCache;
    if (e === null) {
      e = l.pingCache = new cy();
      var u = /* @__PURE__ */ new Set();
      e.set(t, u);
    } else
      u = e.get(t), u === void 0 && (u = /* @__PURE__ */ new Set(), e.set(t, u));
    u.has(a) || (Dc = !0, u.add(a), l = vy.bind(null, l, t, a), t.then(l, l));
  }
  function vy(l, t, a) {
    var e = l.pingCache;
    e !== null && e.delete(t), l.pingedLanes |= l.suspendedLanes & a, l.warmLanes &= ~a, vl === l && (w & a) === a && (_l === 4 || _l === 3 && (w & 62914560) === w && 300 > ct() - bn ? (ul & 2) === 0 && Ee(l, 0) : Uc |= a, _e === w && (_e = 0)), qt(l);
  }
  function Ud(l, t) {
    t === 0 && (t = Ef()), l = Ha(l, t), l !== null && (He(l, t), qt(l));
  }
  function yy(l) {
    var t = l.memoizedState, a = 0;
    t !== null && (a = t.retryLane), Ud(l, a);
  }
  function my(l, t) {
    var a = 0;
    switch (l.tag) {
      case 31:
      case 13:
        var e = l.stateNode, u = l.memoizedState;
        u !== null && (a = u.retryLane);
        break;
      case 19:
        e = l.stateNode;
        break;
      case 22:
        e = l.stateNode._retryCache;
        break;
      default:
        throw Error(s(314));
    }
    e !== null && e.delete(t), Ud(l, a);
  }
  function hy(l, t) {
    return Wn(l, t);
  }
  var Mn = null, Ae = null, Cc = !1, On = !1, Yc = !1, ba = 0;
  function qt(l) {
    l !== Ae && l.next === null && (Ae === null ? Mn = Ae = l : Ae = Ae.next = l), On = !0, Cc || (Cc = !0, gy());
  }
  function vu(l, t) {
    if (!Yc && On) {
      Yc = !0;
      do
        for (var a = !1, e = Mn; e !== null; ) {
          if (l !== 0) {
            var u = e.pendingLanes;
            if (u === 0) var n = 0;
            else {
              var i = e.suspendedLanes, c = e.pingedLanes;
              n = (1 << 31 - st(42 | l) + 1) - 1, n &= u & ~(i & ~c), n = n & 201326741 ? n & 201326741 | 1 : n ? n | 2 : 0;
            }
            n !== 0 && (a = !0, Rd(e, n));
          } else
            n = w, n = Nu(
              e,
              e === vl ? n : 0,
              e.cancelPendingCommit !== null || e.timeoutHandle !== -1
            ), (n & 3) === 0 || je(e, n) || (a = !0, Rd(e, n));
          e = e.next;
        }
      while (a);
      Yc = !1;
    }
  }
  function ry() {
    Nd();
  }
  function Nd() {
    On = Cc = !1;
    var l = 0;
    ba !== 0 && Oy() && (l = ba);
    for (var t = ct(), a = null, e = Mn; e !== null; ) {
      var u = e.next, n = jd(e, t);
      n === 0 ? (e.next = null, a === null ? Mn = u : a.next = u, u === null && (Ae = a)) : (a = e, (l !== 0 || (n & 3) !== 0) && (On = !0)), e = u;
    }
    Bl !== 0 && Bl !== 5 || vu(l), ba !== 0 && (ba = 0);
  }
  function jd(l, t) {
    for (var a = l.suspendedLanes, e = l.pingedLanes, u = l.expirationTimes, n = l.pendingLanes & -62914561; 0 < n; ) {
      var i = 31 - st(n), c = 1 << i, f = u[i];
      f === -1 ? ((c & a) === 0 || (c & e) !== 0) && (u[i] = Lv(c, t)) : f <= t && (l.expiredLanes |= c), n &= ~c;
    }
    if (t = vl, a = w, a = Nu(
      l,
      l === t ? a : 0,
      l.cancelPendingCommit !== null || l.timeoutHandle !== -1
    ), e = l.callbackNode, a === 0 || l === t && (cl === 2 || cl === 9) || l.cancelPendingCommit !== null)
      return e !== null && e !== null && $n(e), l.callbackNode = null, l.callbackPriority = 0;
    if ((a & 3) === 0 || je(l, a)) {
      if (t = a & -a, t === l.callbackPriority) return t;
      switch (e !== null && $n(e), Pn(a)) {
        case 2:
        case 8:
          a = _f;
          break;
        case 32:
          a = Mu;
          break;
        case 268435456:
          a = zf;
          break;
        default:
          a = Mu;
      }
      return e = Hd.bind(null, l), a = Wn(a, e), l.callbackPriority = t, l.callbackNode = a, t;
    }
    return e !== null && e !== null && $n(e), l.callbackPriority = 2, l.callbackNode = null, 2;
  }
  function Hd(l, t) {
    if (Bl !== 0 && Bl !== 5)
      return l.callbackNode = null, l.callbackPriority = 0, null;
    var a = l.callbackNode;
    if (pn() && l.callbackNode !== a)
      return null;
    var e = w;
    return e = Nu(
      l,
      l === vl ? e : 0,
      l.cancelPendingCommit !== null || l.timeoutHandle !== -1
    ), e === 0 ? null : (yd(l, e, t), jd(l, ct()), l.callbackNode != null && l.callbackNode === a ? Hd.bind(null, l) : null);
  }
  function Rd(l, t) {
    if (pn()) return null;
    yd(l, t, !0);
  }
  function gy() {
    Uy(function() {
      (ul & 6) !== 0 ? Wn(
        bf,
        ry
      ) : Nd();
    });
  }
  function Gc() {
    if (ba === 0) {
      var l = se;
      l === 0 && (l = Ou, Ou <<= 1, (Ou & 261888) === 0 && (Ou = 256)), ba = l;
    }
    return ba;
  }
  function xd(l) {
    return l == null || typeof l == "symbol" || typeof l == "boolean" ? null : typeof l == "function" ? l : xu("" + l);
  }
  function qd(l, t) {
    var a = t.ownerDocument.createElement("input");
    return a.name = t.name, a.value = t.value, l.id && a.setAttribute("form", l.id), t.parentNode.insertBefore(a, t), l = new FormData(l), a.parentNode.removeChild(a), l;
  }
  function Sy(l, t, a, e, u) {
    if (t === "submit" && a && a.stateNode === u) {
      var n = xd(
        (u[lt] || null).action
      ), i = e.submitter;
      i && (t = (t = i[lt] || null) ? xd(t.formAction) : i.getAttribute("formAction"), t !== null && (n = t, i = null));
      var c = new Yu(
        "action",
        "action",
        null,
        e,
        u
      );
      l.push({
        event: c,
        listeners: [
          {
            instance: null,
            listener: function() {
              if (e.defaultPrevented) {
                if (ba !== 0) {
                  var f = i ? qd(u, i) : new FormData(u);
                  nc(
                    a,
                    {
                      pending: !0,
                      data: f,
                      method: u.method,
                      action: n
                    },
                    null,
                    f
                  );
                }
              } else
                typeof n == "function" && (c.preventDefault(), f = i ? qd(u, i) : new FormData(u), nc(
                  a,
                  {
                    pending: !0,
                    data: f,
                    method: u.method,
                    action: n
                  },
                  n,
                  f
                ));
            },
            currentTarget: u
          }
        ]
      });
    }
  }
  for (var Xc = 0; Xc < Ei.length; Xc++) {
    var Qc = Ei[Xc], by = Qc.toLowerCase(), _y = Qc[0].toUpperCase() + Qc.slice(1);
    Ut(
      by,
      "on" + _y
    );
  }
  Ut(ds, "onAnimationEnd"), Ut(vs, "onAnimationIteration"), Ut(ys, "onAnimationStart"), Ut("dblclick", "onDoubleClick"), Ut("focusin", "onFocus"), Ut("focusout", "onBlur"), Ut(B0, "onTransitionRun"), Ut(C0, "onTransitionStart"), Ut(Y0, "onTransitionCancel"), Ut(ms, "onTransitionEnd"), Wa("onMouseEnter", ["mouseout", "mouseover"]), Wa("onMouseLeave", ["mouseout", "mouseover"]), Wa("onPointerEnter", ["pointerout", "pointerover"]), Wa("onPointerLeave", ["pointerout", "pointerover"]), Da(
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
  var yu = "abort canplay canplaythrough durationchange emptied encrypted ended error loadeddata loadedmetadata loadstart pause play playing progress ratechange resize seeked seeking stalled suspend timeupdate volumechange waiting".split(
    " "
  ), zy = new Set(
    "beforetoggle cancel close invalid load scroll scrollend toggle".split(" ").concat(yu)
  );
  function Bd(l, t) {
    t = (t & 4) !== 0;
    for (var a = 0; a < l.length; a++) {
      var e = l[a], u = e.event;
      e = e.listeners;
      l: {
        var n = void 0;
        if (t)
          for (var i = e.length - 1; 0 <= i; i--) {
            var c = e[i], f = c.instance, h = c.currentTarget;
            if (c = c.listener, f !== n && u.isPropagationStopped())
              break l;
            n = c, u.currentTarget = h;
            try {
              n(u);
            } catch (S) {
              Qu(S);
            }
            u.currentTarget = null, n = f;
          }
        else
          for (i = 0; i < e.length; i++) {
            if (c = e[i], f = c.instance, h = c.currentTarget, c = c.listener, f !== n && u.isPropagationStopped())
              break l;
            n = c, u.currentTarget = h;
            try {
              n(u);
            } catch (S) {
              Qu(S);
            }
            u.currentTarget = null, n = f;
          }
      }
    }
  }
  function J(l, t) {
    var a = t[li];
    a === void 0 && (a = t[li] = /* @__PURE__ */ new Set());
    var e = l + "__bubble";
    a.has(e) || (Cd(t, l, 2, !1), a.add(e));
  }
  function Zc(l, t, a) {
    var e = 0;
    t && (e |= 4), Cd(
      a,
      l,
      e,
      t
    );
  }
  var Dn = "_reactListening" + Math.random().toString(36).slice(2);
  function Lc(l) {
    if (!l[Dn]) {
      l[Dn] = !0, Uf.forEach(function(a) {
        a !== "selectionchange" && (zy.has(a) || Zc(a, !1, l), Zc(a, !0, l));
      });
      var t = l.nodeType === 9 ? l : l.ownerDocument;
      t === null || t[Dn] || (t[Dn] = !0, Zc("selectionchange", !1, t));
    }
  }
  function Cd(l, t, a, e) {
    switch (vv(t)) {
      case 2:
        var u = Wy;
        break;
      case 8:
        u = $y;
        break;
      default:
        u = uf;
    }
    a = u.bind(
      null,
      t,
      a,
      l
    ), u = void 0, !si || t !== "touchstart" && t !== "touchmove" && t !== "wheel" || (u = !0), e ? u !== void 0 ? l.addEventListener(t, a, {
      capture: !0,
      passive: u
    }) : l.addEventListener(t, a, !0) : u !== void 0 ? l.addEventListener(t, a, {
      passive: u
    }) : l.addEventListener(t, a, !1);
  }
  function Vc(l, t, a, e, u) {
    var n = e;
    if ((t & 1) === 0 && (t & 2) === 0 && e !== null)
      l: for (; ; ) {
        if (e === null) return;
        var i = e.tag;
        if (i === 3 || i === 4) {
          var c = e.stateNode.containerInfo;
          if (c === u) break;
          if (i === 4)
            for (i = e.return; i !== null; ) {
              var f = i.tag;
              if ((f === 3 || f === 4) && i.stateNode.containerInfo === u)
                return;
              i = i.return;
            }
          for (; c !== null; ) {
            if (i = Ja(c), i === null) return;
            if (f = i.tag, f === 5 || f === 6 || f === 26 || f === 27) {
              e = n = i;
              continue l;
            }
            c = c.parentNode;
          }
        }
        e = e.return;
      }
    Qf(function() {
      var h = n, S = ci(a), E = [];
      l: {
        var r = hs.get(l);
        if (r !== void 0) {
          var g = Yu, N = l;
          switch (l) {
            case "keypress":
              if (Bu(a) === 0) break l;
            case "keydown":
            case "keyup":
              g = m0;
              break;
            case "focusin":
              N = "focus", g = yi;
              break;
            case "focusout":
              N = "blur", g = yi;
              break;
            case "beforeblur":
            case "afterblur":
              g = yi;
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
              g = a0;
              break;
            case "touchcancel":
            case "touchend":
            case "touchmove":
            case "touchstart":
              g = g0;
              break;
            case ds:
            case vs:
            case ys:
              g = n0;
              break;
            case ms:
              g = b0;
              break;
            case "scroll":
            case "scrollend":
              g = l0;
              break;
            case "wheel":
              g = z0;
              break;
            case "copy":
            case "cut":
            case "paste":
              g = c0;
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
              g = T0;
          }
          var C = (t & 4) !== 0, dl = !C && (l === "scroll" || l === "scrollend"), y = C ? r !== null ? r + "Capture" : null : r;
          C = [];
          for (var o = h, m; o !== null; ) {
            var z = o;
            if (m = z.stateNode, z = z.tag, z !== 5 && z !== 26 && z !== 27 || m === null || y === null || (z = qe(o, y), z != null && C.push(
              mu(o, z, m)
            )), dl) break;
            o = o.return;
          }
          0 < C.length && (r = new g(
            r,
            N,
            null,
            a,
            S
          ), E.push({ event: r, listeners: C }));
        }
      }
      if ((t & 7) === 0) {
        l: {
          if (r = l === "mouseover" || l === "pointerover", g = l === "mouseout" || l === "pointerout", r && a !== ii && (N = a.relatedTarget || a.fromElement) && (Ja(N) || N[Ka]))
            break l;
          if ((g || r) && (r = S.window === S ? S : (r = S.ownerDocument) ? r.defaultView || r.parentWindow : window, g ? (N = a.relatedTarget || a.toElement, g = h, N = N ? Ja(N) : null, N !== null && (dl = W(N), C = N.tag, N !== dl || C !== 5 && C !== 27 && C !== 6) && (N = null)) : (g = null, N = h), g !== N)) {
            if (C = Vf, z = "onMouseLeave", y = "onMouseEnter", o = "mouse", (l === "pointerout" || l === "pointerover") && (C = Jf, z = "onPointerLeave", y = "onPointerEnter", o = "pointer"), dl = g == null ? r : xe(g), m = N == null ? r : xe(N), r = new C(
              z,
              o + "leave",
              g,
              a,
              S
            ), r.target = dl, r.relatedTarget = m, z = null, Ja(S) === h && (C = new C(
              y,
              o + "enter",
              N,
              a,
              S
            ), C.target = m, C.relatedTarget = dl, z = C), dl = z, g && N)
              t: {
                for (C = Ey, y = g, o = N, m = 0, z = y; z; z = C(z))
                  m++;
                z = 0;
                for (var B = o; B; B = C(B))
                  z++;
                for (; 0 < m - z; )
                  y = C(y), m--;
                for (; 0 < z - m; )
                  o = C(o), z--;
                for (; m--; ) {
                  if (y === o || o !== null && y === o.alternate) {
                    C = y;
                    break t;
                  }
                  y = C(y), o = C(o);
                }
                C = null;
              }
            else C = null;
            g !== null && Yd(
              E,
              r,
              g,
              C,
              !1
            ), N !== null && dl !== null && Yd(
              E,
              dl,
              N,
              C,
              !0
            );
          }
        }
        l: {
          if (r = h ? xe(h) : window, g = r.nodeName && r.nodeName.toLowerCase(), g === "select" || g === "input" && r.type === "file")
            var ll = ls;
          else if (If(r))
            if (ts)
              ll = R0;
            else {
              ll = j0;
              var H = N0;
            }
          else
            g = r.nodeName, !g || g.toLowerCase() !== "input" || r.type !== "checkbox" && r.type !== "radio" ? h && ni(h.elementType) && (ll = ls) : ll = H0;
          if (ll && (ll = ll(l, h))) {
            Pf(
              E,
              ll,
              a,
              S
            );
            break l;
          }
          H && H(l, r, h), l === "focusout" && h && r.type === "number" && h.memoizedProps.value != null && ui(r, "number", r.value);
        }
        switch (H = h ? xe(h) : window, l) {
          case "focusin":
            (If(H) || H.contentEditable === "true") && (te = H, bi = h, Le = null);
            break;
          case "focusout":
            Le = bi = te = null;
            break;
          case "mousedown":
            _i = !0;
            break;
          case "contextmenu":
          case "mouseup":
          case "dragend":
            _i = !1, ss(E, a, S);
            break;
          case "selectionchange":
            if (q0) break;
          case "keydown":
          case "keyup":
            ss(E, a, S);
        }
        var Z;
        if (hi)
          l: {
            switch (l) {
              case "compositionstart":
                var k = "onCompositionStart";
                break l;
              case "compositionend":
                k = "onCompositionEnd";
                break l;
              case "compositionupdate":
                k = "onCompositionUpdate";
                break l;
            }
            k = void 0;
          }
        else
          le ? $f(l, a) && (k = "onCompositionEnd") : l === "keydown" && a.keyCode === 229 && (k = "onCompositionStart");
        k && (wf && a.locale !== "ko" && (le || k !== "onCompositionStart" ? k === "onCompositionEnd" && le && (Z = Zf()) : (ea = S, oi = "value" in ea ? ea.value : ea.textContent, le = !0)), H = Un(h, k), 0 < H.length && (k = new Kf(
          k,
          l,
          null,
          a,
          S
        ), E.push({ event: k, listeners: H }), Z ? k.data = Z : (Z = Ff(a), Z !== null && (k.data = Z)))), (Z = p0 ? M0(l, a) : O0(l, a)) && (k = Un(h, "onBeforeInput"), 0 < k.length && (H = new Kf(
          "onBeforeInput",
          "beforeinput",
          null,
          a,
          S
        ), E.push({
          event: H,
          listeners: k
        }), H.data = Z)), Sy(
          E,
          l,
          h,
          a,
          S
        );
      }
      Bd(E, t);
    });
  }
  function mu(l, t, a) {
    return {
      instance: l,
      listener: t,
      currentTarget: a
    };
  }
  function Un(l, t) {
    for (var a = t + "Capture", e = []; l !== null; ) {
      var u = l, n = u.stateNode;
      if (u = u.tag, u !== 5 && u !== 26 && u !== 27 || n === null || (u = qe(l, a), u != null && e.unshift(
        mu(l, u, n)
      ), u = qe(l, t), u != null && e.push(
        mu(l, u, n)
      )), l.tag === 3) return e;
      l = l.return;
    }
    return [];
  }
  function Ey(l) {
    if (l === null) return null;
    do
      l = l.return;
    while (l && l.tag !== 5 && l.tag !== 27);
    return l || null;
  }
  function Yd(l, t, a, e, u) {
    for (var n = t._reactName, i = []; a !== null && a !== e; ) {
      var c = a, f = c.alternate, h = c.stateNode;
      if (c = c.tag, f !== null && f === e) break;
      c !== 5 && c !== 26 && c !== 27 || h === null || (f = h, u ? (h = qe(a, n), h != null && i.unshift(
        mu(a, h, f)
      )) : u || (h = qe(a, n), h != null && i.push(
        mu(a, h, f)
      ))), a = a.return;
    }
    i.length !== 0 && l.push({ event: t, listeners: i });
  }
  var Ty = /\r\n?/g, Ay = /\u0000|\uFFFD/g;
  function Gd(l) {
    return (typeof l == "string" ? l : "" + l).replace(Ty, `
`).replace(Ay, "");
  }
  function Xd(l, t) {
    return t = Gd(t), Gd(l) === t;
  }
  function ol(l, t, a, e, u, n) {
    switch (a) {
      case "children":
        typeof e == "string" ? t === "body" || t === "textarea" && e === "" || Fa(l, e) : (typeof e == "number" || typeof e == "bigint") && t !== "body" && Fa(l, "" + e);
        break;
      case "className":
        Hu(l, "class", e);
        break;
      case "tabIndex":
        Hu(l, "tabindex", e);
        break;
      case "dir":
      case "role":
      case "viewBox":
      case "width":
      case "height":
        Hu(l, a, e);
        break;
      case "style":
        Gf(l, e, n);
        break;
      case "data":
        if (t !== "object") {
          Hu(l, "data", e);
          break;
        }
      case "src":
      case "href":
        if (e === "" && (t !== "a" || a !== "href")) {
          l.removeAttribute(a);
          break;
        }
        if (e == null || typeof e == "function" || typeof e == "symbol" || typeof e == "boolean") {
          l.removeAttribute(a);
          break;
        }
        e = xu("" + e), l.setAttribute(a, e);
        break;
      case "action":
      case "formAction":
        if (typeof e == "function") {
          l.setAttribute(
            a,
            "javascript:throw new Error('A React form was unexpectedly submitted. If you called form.submit() manually, consider using form.requestSubmit() instead. If you\\'re trying to use event.stopPropagation() in a submit event handler, consider also calling event.preventDefault().')"
          );
          break;
        } else
          typeof n == "function" && (a === "formAction" ? (t !== "input" && ol(l, t, "name", u.name, u, null), ol(
            l,
            t,
            "formEncType",
            u.formEncType,
            u,
            null
          ), ol(
            l,
            t,
            "formMethod",
            u.formMethod,
            u,
            null
          ), ol(
            l,
            t,
            "formTarget",
            u.formTarget,
            u,
            null
          )) : (ol(l, t, "encType", u.encType, u, null), ol(l, t, "method", u.method, u, null), ol(l, t, "target", u.target, u, null)));
        if (e == null || typeof e == "symbol" || typeof e == "boolean") {
          l.removeAttribute(a);
          break;
        }
        e = xu("" + e), l.setAttribute(a, e);
        break;
      case "onClick":
        e != null && (l.onclick = Yt);
        break;
      case "onScroll":
        e != null && J("scroll", l);
        break;
      case "onScrollEnd":
        e != null && J("scrollend", l);
        break;
      case "dangerouslySetInnerHTML":
        if (e != null) {
          if (typeof e != "object" || !("__html" in e))
            throw Error(s(61));
          if (a = e.__html, a != null) {
            if (u.children != null) throw Error(s(60));
            l.innerHTML = a;
          }
        }
        break;
      case "multiple":
        l.multiple = e && typeof e != "function" && typeof e != "symbol";
        break;
      case "muted":
        l.muted = e && typeof e != "function" && typeof e != "symbol";
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
        if (e == null || typeof e == "function" || typeof e == "boolean" || typeof e == "symbol") {
          l.removeAttribute("xlink:href");
          break;
        }
        a = xu("" + e), l.setAttributeNS(
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
        e != null && typeof e != "function" && typeof e != "symbol" ? l.setAttribute(a, "" + e) : l.removeAttribute(a);
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
        e && typeof e != "function" && typeof e != "symbol" ? l.setAttribute(a, "") : l.removeAttribute(a);
        break;
      case "capture":
      case "download":
        e === !0 ? l.setAttribute(a, "") : e !== !1 && e != null && typeof e != "function" && typeof e != "symbol" ? l.setAttribute(a, e) : l.removeAttribute(a);
        break;
      case "cols":
      case "rows":
      case "size":
      case "span":
        e != null && typeof e != "function" && typeof e != "symbol" && !isNaN(e) && 1 <= e ? l.setAttribute(a, e) : l.removeAttribute(a);
        break;
      case "rowSpan":
      case "start":
        e == null || typeof e == "function" || typeof e == "symbol" || isNaN(e) ? l.removeAttribute(a) : l.setAttribute(a, e);
        break;
      case "popover":
        J("beforetoggle", l), J("toggle", l), ju(l, "popover", e);
        break;
      case "xlinkActuate":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:actuate",
          e
        );
        break;
      case "xlinkArcrole":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:arcrole",
          e
        );
        break;
      case "xlinkRole":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:role",
          e
        );
        break;
      case "xlinkShow":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:show",
          e
        );
        break;
      case "xlinkTitle":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:title",
          e
        );
        break;
      case "xlinkType":
        Ct(
          l,
          "http://www.w3.org/1999/xlink",
          "xlink:type",
          e
        );
        break;
      case "xmlBase":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:base",
          e
        );
        break;
      case "xmlLang":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:lang",
          e
        );
        break;
      case "xmlSpace":
        Ct(
          l,
          "http://www.w3.org/XML/1998/namespace",
          "xml:space",
          e
        );
        break;
      case "is":
        ju(l, "is", e);
        break;
      case "innerText":
      case "textContent":
        break;
      default:
        (!(2 < a.length) || a[0] !== "o" && a[0] !== "O" || a[1] !== "n" && a[1] !== "N") && (a = Iv.get(a) || a, ju(l, a, e));
    }
  }
  function Kc(l, t, a, e, u, n) {
    switch (a) {
      case "style":
        Gf(l, e, n);
        break;
      case "dangerouslySetInnerHTML":
        if (e != null) {
          if (typeof e != "object" || !("__html" in e))
            throw Error(s(61));
          if (a = e.__html, a != null) {
            if (u.children != null) throw Error(s(60));
            l.innerHTML = a;
          }
        }
        break;
      case "children":
        typeof e == "string" ? Fa(l, e) : (typeof e == "number" || typeof e == "bigint") && Fa(l, "" + e);
        break;
      case "onScroll":
        e != null && J("scroll", l);
        break;
      case "onScrollEnd":
        e != null && J("scrollend", l);
        break;
      case "onClick":
        e != null && (l.onclick = Yt);
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
            if (a[0] === "o" && a[1] === "n" && (u = a.endsWith("Capture"), t = a.slice(2, u ? a.length - 7 : void 0), n = l[lt] || null, n = n != null ? n[a] : null, typeof n == "function" && l.removeEventListener(t, n, u), typeof e == "function")) {
              typeof n != "function" && n !== null && (a in l ? l[a] = null : l.hasAttribute(a) && l.removeAttribute(a)), l.addEventListener(t, e, u);
              break l;
            }
            a in l ? l[a] = e : e === !0 ? l.setAttribute(a, "") : ju(l, a, e);
          }
    }
  }
  function Kl(l, t, a) {
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
        var e = !1, u = !1, n;
        for (n in a)
          if (a.hasOwnProperty(n)) {
            var i = a[n];
            if (i != null)
              switch (n) {
                case "src":
                  e = !0;
                  break;
                case "srcSet":
                  u = !0;
                  break;
                case "children":
                case "dangerouslySetInnerHTML":
                  throw Error(s(137, t));
                default:
                  ol(l, t, n, i, a, null);
              }
          }
        u && ol(l, t, "srcSet", a.srcSet, a, null), e && ol(l, t, "src", a.src, a, null);
        return;
      case "input":
        J("invalid", l);
        var c = n = i = u = null, f = null, h = null;
        for (e in a)
          if (a.hasOwnProperty(e)) {
            var S = a[e];
            if (S != null)
              switch (e) {
                case "name":
                  u = S;
                  break;
                case "type":
                  i = S;
                  break;
                case "checked":
                  f = S;
                  break;
                case "defaultChecked":
                  h = S;
                  break;
                case "value":
                  n = S;
                  break;
                case "defaultValue":
                  c = S;
                  break;
                case "children":
                case "dangerouslySetInnerHTML":
                  if (S != null)
                    throw Error(s(137, t));
                  break;
                default:
                  ol(l, t, e, S, a, null);
              }
          }
        qf(
          l,
          n,
          c,
          f,
          h,
          i,
          u,
          !1
        );
        return;
      case "select":
        J("invalid", l), e = i = n = null;
        for (u in a)
          if (a.hasOwnProperty(u) && (c = a[u], c != null))
            switch (u) {
              case "value":
                n = c;
                break;
              case "defaultValue":
                i = c;
                break;
              case "multiple":
                e = c;
              default:
                ol(l, t, u, c, a, null);
            }
        t = n, a = i, l.multiple = !!e, t != null ? $a(l, !!e, t, !1) : a != null && $a(l, !!e, a, !0);
        return;
      case "textarea":
        J("invalid", l), n = u = e = null;
        for (i in a)
          if (a.hasOwnProperty(i) && (c = a[i], c != null))
            switch (i) {
              case "value":
                e = c;
                break;
              case "defaultValue":
                u = c;
                break;
              case "children":
                n = c;
                break;
              case "dangerouslySetInnerHTML":
                if (c != null) throw Error(s(91));
                break;
              default:
                ol(l, t, i, c, a, null);
            }
        Cf(l, e, u, n);
        return;
      case "option":
        for (f in a)
          if (a.hasOwnProperty(f) && (e = a[f], e != null))
            switch (f) {
              case "selected":
                l.selected = e && typeof e != "function" && typeof e != "symbol";
                break;
              default:
                ol(l, t, f, e, a, null);
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
        for (e = 0; e < yu.length; e++)
          J(yu[e], l);
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
        for (h in a)
          if (a.hasOwnProperty(h) && (e = a[h], e != null))
            switch (h) {
              case "children":
              case "dangerouslySetInnerHTML":
                throw Error(s(137, t));
              default:
                ol(l, t, h, e, a, null);
            }
        return;
      default:
        if (ni(t)) {
          for (S in a)
            a.hasOwnProperty(S) && (e = a[S], e !== void 0 && Kc(
              l,
              t,
              S,
              e,
              a,
              void 0
            ));
          return;
        }
    }
    for (c in a)
      a.hasOwnProperty(c) && (e = a[c], e != null && ol(l, t, c, e, a, null));
  }
  function py(l, t, a, e) {
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
        var u = null, n = null, i = null, c = null, f = null, h = null, S = null;
        for (g in a) {
          var E = a[g];
          if (a.hasOwnProperty(g) && E != null)
            switch (g) {
              case "checked":
                break;
              case "value":
                break;
              case "defaultValue":
                f = E;
              default:
                e.hasOwnProperty(g) || ol(l, t, g, null, e, E);
            }
        }
        for (var r in e) {
          var g = e[r];
          if (E = a[r], e.hasOwnProperty(r) && (g != null || E != null))
            switch (r) {
              case "type":
                n = g;
                break;
              case "name":
                u = g;
                break;
              case "checked":
                h = g;
                break;
              case "defaultChecked":
                S = g;
                break;
              case "value":
                i = g;
                break;
              case "defaultValue":
                c = g;
                break;
              case "children":
              case "dangerouslySetInnerHTML":
                if (g != null)
                  throw Error(s(137, t));
                break;
              default:
                g !== E && ol(
                  l,
                  t,
                  r,
                  g,
                  e,
                  E
                );
            }
        }
        ei(
          l,
          i,
          c,
          f,
          h,
          S,
          n,
          u
        );
        return;
      case "select":
        g = i = c = r = null;
        for (n in a)
          if (f = a[n], a.hasOwnProperty(n) && f != null)
            switch (n) {
              case "value":
                break;
              case "multiple":
                g = f;
              default:
                e.hasOwnProperty(n) || ol(
                  l,
                  t,
                  n,
                  null,
                  e,
                  f
                );
            }
        for (u in e)
          if (n = e[u], f = a[u], e.hasOwnProperty(u) && (n != null || f != null))
            switch (u) {
              case "value":
                r = n;
                break;
              case "defaultValue":
                c = n;
                break;
              case "multiple":
                i = n;
              default:
                n !== f && ol(
                  l,
                  t,
                  u,
                  n,
                  e,
                  f
                );
            }
        t = c, a = i, e = g, r != null ? $a(l, !!a, r, !1) : !!e != !!a && (t != null ? $a(l, !!a, t, !0) : $a(l, !!a, a ? [] : "", !1));
        return;
      case "textarea":
        g = r = null;
        for (c in a)
          if (u = a[c], a.hasOwnProperty(c) && u != null && !e.hasOwnProperty(c))
            switch (c) {
              case "value":
                break;
              case "children":
                break;
              default:
                ol(l, t, c, null, e, u);
            }
        for (i in e)
          if (u = e[i], n = a[i], e.hasOwnProperty(i) && (u != null || n != null))
            switch (i) {
              case "value":
                r = u;
                break;
              case "defaultValue":
                g = u;
                break;
              case "children":
                break;
              case "dangerouslySetInnerHTML":
                if (u != null) throw Error(s(91));
                break;
              default:
                u !== n && ol(l, t, i, u, e, n);
            }
        Bf(l, r, g);
        return;
      case "option":
        for (var N in a)
          if (r = a[N], a.hasOwnProperty(N) && r != null && !e.hasOwnProperty(N))
            switch (N) {
              case "selected":
                l.selected = !1;
                break;
              default:
                ol(
                  l,
                  t,
                  N,
                  null,
                  e,
                  r
                );
            }
        for (f in e)
          if (r = e[f], g = a[f], e.hasOwnProperty(f) && r !== g && (r != null || g != null))
            switch (f) {
              case "selected":
                l.selected = r && typeof r != "function" && typeof r != "symbol";
                break;
              default:
                ol(
                  l,
                  t,
                  f,
                  r,
                  e,
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
        for (var C in a)
          r = a[C], a.hasOwnProperty(C) && r != null && !e.hasOwnProperty(C) && ol(l, t, C, null, e, r);
        for (h in e)
          if (r = e[h], g = a[h], e.hasOwnProperty(h) && r !== g && (r != null || g != null))
            switch (h) {
              case "children":
              case "dangerouslySetInnerHTML":
                if (r != null)
                  throw Error(s(137, t));
                break;
              default:
                ol(
                  l,
                  t,
                  h,
                  r,
                  e,
                  g
                );
            }
        return;
      default:
        if (ni(t)) {
          for (var dl in a)
            r = a[dl], a.hasOwnProperty(dl) && r !== void 0 && !e.hasOwnProperty(dl) && Kc(
              l,
              t,
              dl,
              void 0,
              e,
              r
            );
          for (S in e)
            r = e[S], g = a[S], !e.hasOwnProperty(S) || r === g || r === void 0 && g === void 0 || Kc(
              l,
              t,
              S,
              r,
              e,
              g
            );
          return;
        }
    }
    for (var y in a)
      r = a[y], a.hasOwnProperty(y) && r != null && !e.hasOwnProperty(y) && ol(l, t, y, null, e, r);
    for (E in e)
      r = e[E], g = a[E], !e.hasOwnProperty(E) || r === g || r == null && g == null || ol(l, t, E, r, e, g);
  }
  function Qd(l) {
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
  function My() {
    if (typeof performance.getEntriesByType == "function") {
      for (var l = 0, t = 0, a = performance.getEntriesByType("resource"), e = 0; e < a.length; e++) {
        var u = a[e], n = u.transferSize, i = u.initiatorType, c = u.duration;
        if (n && c && Qd(i)) {
          for (i = 0, c = u.responseEnd, e += 1; e < a.length; e++) {
            var f = a[e], h = f.startTime;
            if (h > c) break;
            var S = f.transferSize, E = f.initiatorType;
            S && Qd(E) && (f = f.responseEnd, i += S * (f < c ? 1 : (c - h) / (f - h)));
          }
          if (--e, t += 8 * (n + i) / (u.duration / 1e3), l++, 10 < l) break;
        }
      }
      if (0 < l) return t / l / 1e6;
    }
    return navigator.connection && (l = navigator.connection.downlink, typeof l == "number") ? l : 5;
  }
  var Jc = null, wc = null;
  function Nn(l) {
    return l.nodeType === 9 ? l : l.ownerDocument;
  }
  function Zd(l) {
    switch (l) {
      case "http://www.w3.org/2000/svg":
        return 1;
      case "http://www.w3.org/1998/Math/MathML":
        return 2;
      default:
        return 0;
    }
  }
  function Ld(l, t) {
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
  function kc(l, t) {
    return l === "textarea" || l === "noscript" || typeof t.children == "string" || typeof t.children == "number" || typeof t.children == "bigint" || typeof t.dangerouslySetInnerHTML == "object" && t.dangerouslySetInnerHTML !== null && t.dangerouslySetInnerHTML.__html != null;
  }
  var Wc = null;
  function Oy() {
    var l = window.event;
    return l && l.type === "popstate" ? l === Wc ? !1 : (Wc = l, !0) : (Wc = null, !1);
  }
  var Vd = typeof setTimeout == "function" ? setTimeout : void 0, Dy = typeof clearTimeout == "function" ? clearTimeout : void 0, Kd = typeof Promise == "function" ? Promise : void 0, Uy = typeof queueMicrotask == "function" ? queueMicrotask : typeof Kd < "u" ? function(l) {
    return Kd.resolve(null).then(l).catch(Ny);
  } : Vd;
  function Ny(l) {
    setTimeout(function() {
      throw l;
    });
  }
  function _a(l) {
    return l === "head";
  }
  function Jd(l, t) {
    var a = t, e = 0;
    do {
      var u = a.nextSibling;
      if (l.removeChild(a), u && u.nodeType === 8)
        if (a = u.data, a === "/$" || a === "/&") {
          if (e === 0) {
            l.removeChild(u), De(t);
            return;
          }
          e--;
        } else if (a === "$" || a === "$?" || a === "$~" || a === "$!" || a === "&")
          e++;
        else if (a === "html")
          hu(l.ownerDocument.documentElement);
        else if (a === "head") {
          a = l.ownerDocument.head, hu(a);
          for (var n = a.firstChild; n; ) {
            var i = n.nextSibling, c = n.nodeName;
            n[Re] || c === "SCRIPT" || c === "STYLE" || c === "LINK" && n.rel.toLowerCase() === "stylesheet" || a.removeChild(n), n = i;
          }
        } else
          a === "body" && hu(l.ownerDocument.body);
      a = u;
    } while (a);
    De(t);
  }
  function wd(l, t) {
    var a = l;
    l = 0;
    do {
      var e = a.nextSibling;
      if (a.nodeType === 1 ? t ? (a._stashedDisplay = a.style.display, a.style.display = "none") : (a.style.display = a._stashedDisplay || "", a.getAttribute("style") === "" && a.removeAttribute("style")) : a.nodeType === 3 && (t ? (a._stashedText = a.nodeValue, a.nodeValue = "") : a.nodeValue = a._stashedText || ""), e && e.nodeType === 8)
        if (a = e.data, a === "/$") {
          if (l === 0) break;
          l--;
        } else
          a !== "$" && a !== "$?" && a !== "$~" && a !== "$!" || l++;
      a = e;
    } while (a);
  }
  function $c(l) {
    var t = l.firstChild;
    for (t && t.nodeType === 10 && (t = t.nextSibling); t; ) {
      var a = t;
      switch (t = t.nextSibling, a.nodeName) {
        case "HTML":
        case "HEAD":
        case "BODY":
          $c(a), ti(a);
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
  function jy(l, t, a, e) {
    for (; l.nodeType === 1; ) {
      var u = a;
      if (l.nodeName.toLowerCase() !== t.toLowerCase()) {
        if (!e && (l.nodeName !== "INPUT" || l.type !== "hidden"))
          break;
      } else if (e) {
        if (!l[Re])
          switch (t) {
            case "meta":
              if (!l.hasAttribute("itemprop")) break;
              return l;
            case "link":
              if (n = l.getAttribute("rel"), n === "stylesheet" && l.hasAttribute("data-precedence"))
                break;
              if (n !== u.rel || l.getAttribute("href") !== (u.href == null || u.href === "" ? null : u.href) || l.getAttribute("crossorigin") !== (u.crossOrigin == null ? null : u.crossOrigin) || l.getAttribute("title") !== (u.title == null ? null : u.title))
                break;
              return l;
            case "style":
              if (l.hasAttribute("data-precedence")) break;
              return l;
            case "script":
              if (n = l.getAttribute("src"), (n !== (u.src == null ? null : u.src) || l.getAttribute("type") !== (u.type == null ? null : u.type) || l.getAttribute("crossorigin") !== (u.crossOrigin == null ? null : u.crossOrigin)) && n && l.hasAttribute("async") && !l.hasAttribute("itemprop"))
                break;
              return l;
            default:
              return l;
          }
      } else if (t === "input" && l.type === "hidden") {
        var n = u.name == null ? null : "" + u.name;
        if (u.type === "hidden" && l.getAttribute("name") === n)
          return l;
      } else return l;
      if (l = Ot(l.nextSibling), l === null) break;
    }
    return null;
  }
  function Hy(l, t, a) {
    if (t === "") return null;
    for (; l.nodeType !== 3; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !a || (l = Ot(l.nextSibling), l === null)) return null;
    return l;
  }
  function kd(l, t) {
    for (; l.nodeType !== 8; )
      if ((l.nodeType !== 1 || l.nodeName !== "INPUT" || l.type !== "hidden") && !t || (l = Ot(l.nextSibling), l === null)) return null;
    return l;
  }
  function Fc(l) {
    return l.data === "$?" || l.data === "$~";
  }
  function Ic(l) {
    return l.data === "$!" || l.data === "$?" && l.ownerDocument.readyState !== "loading";
  }
  function Ry(l, t) {
    var a = l.ownerDocument;
    if (l.data === "$~") l._reactRetry = t;
    else if (l.data !== "$?" || a.readyState !== "loading")
      t();
    else {
      var e = function() {
        t(), a.removeEventListener("DOMContentLoaded", e);
      };
      a.addEventListener("DOMContentLoaded", e), l._reactRetry = e;
    }
  }
  function Ot(l) {
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
  var Pc = null;
  function Wd(l) {
    l = l.nextSibling;
    for (var t = 0; l; ) {
      if (l.nodeType === 8) {
        var a = l.data;
        if (a === "/$" || a === "/&") {
          if (t === 0)
            return Ot(l.nextSibling);
          t--;
        } else
          a !== "$" && a !== "$!" && a !== "$?" && a !== "$~" && a !== "&" || t++;
      }
      l = l.nextSibling;
    }
    return null;
  }
  function $d(l) {
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
  function Fd(l, t, a) {
    switch (t = Nn(a), l) {
      case "html":
        if (l = t.documentElement, !l) throw Error(s(452));
        return l;
      case "head":
        if (l = t.head, !l) throw Error(s(453));
        return l;
      case "body":
        if (l = t.body, !l) throw Error(s(454));
        return l;
      default:
        throw Error(s(451));
    }
  }
  function hu(l) {
    for (var t = l.attributes; t.length; )
      l.removeAttributeNode(t[0]);
    ti(l);
  }
  var Dt = /* @__PURE__ */ new Map(), Id = /* @__PURE__ */ new Set();
  function jn(l) {
    return typeof l.getRootNode == "function" ? l.getRootNode() : l.nodeType === 9 ? l : l.ownerDocument;
  }
  var la = _.d;
  _.d = {
    f: xy,
    r: qy,
    D: By,
    C: Cy,
    L: Yy,
    m: Gy,
    X: Qy,
    S: Xy,
    M: Zy
  };
  function xy() {
    var l = la.f(), t = En();
    return l || t;
  }
  function qy(l) {
    var t = wa(l);
    t !== null && t.tag === 5 && t.type === "form" ? ho(t) : la.r(l);
  }
  var pe = typeof document > "u" ? null : document;
  function Pd(l, t, a) {
    var e = pe;
    if (e && typeof t == "string" && t) {
      var u = _t(t);
      u = 'link[rel="' + l + '"][href="' + u + '"]', typeof a == "string" && (u += '[crossorigin="' + a + '"]'), Id.has(u) || (Id.add(u), l = { rel: l, crossOrigin: a, href: t }, e.querySelector(u) === null && (t = e.createElement("link"), Kl(t, "link", l), Yl(t), e.head.appendChild(t)));
    }
  }
  function By(l) {
    la.D(l), Pd("dns-prefetch", l, null);
  }
  function Cy(l, t) {
    la.C(l, t), Pd("preconnect", l, t);
  }
  function Yy(l, t, a) {
    la.L(l, t, a);
    var e = pe;
    if (e && l && t) {
      var u = 'link[rel="preload"][as="' + _t(t) + '"]';
      t === "image" && a && a.imageSrcSet ? (u += '[imagesrcset="' + _t(
        a.imageSrcSet
      ) + '"]', typeof a.imageSizes == "string" && (u += '[imagesizes="' + _t(
        a.imageSizes
      ) + '"]')) : u += '[href="' + _t(l) + '"]';
      var n = u;
      switch (t) {
        case "style":
          n = Me(l);
          break;
        case "script":
          n = Oe(l);
      }
      Dt.has(n) || (l = q(
        {
          rel: "preload",
          href: t === "image" && a && a.imageSrcSet ? void 0 : l,
          as: t
        },
        a
      ), Dt.set(n, l), e.querySelector(u) !== null || t === "style" && e.querySelector(ru(n)) || t === "script" && e.querySelector(gu(n)) || (t = e.createElement("link"), Kl(t, "link", l), Yl(t), e.head.appendChild(t)));
    }
  }
  function Gy(l, t) {
    la.m(l, t);
    var a = pe;
    if (a && l) {
      var e = t && typeof t.as == "string" ? t.as : "script", u = 'link[rel="modulepreload"][as="' + _t(e) + '"][href="' + _t(l) + '"]', n = u;
      switch (e) {
        case "audioworklet":
        case "paintworklet":
        case "serviceworker":
        case "sharedworker":
        case "worker":
        case "script":
          n = Oe(l);
      }
      if (!Dt.has(n) && (l = q({ rel: "modulepreload", href: l }, t), Dt.set(n, l), a.querySelector(u) === null)) {
        switch (e) {
          case "audioworklet":
          case "paintworklet":
          case "serviceworker":
          case "sharedworker":
          case "worker":
          case "script":
            if (a.querySelector(gu(n)))
              return;
        }
        e = a.createElement("link"), Kl(e, "link", l), Yl(e), a.head.appendChild(e);
      }
    }
  }
  function Xy(l, t, a) {
    la.S(l, t, a);
    var e = pe;
    if (e && l) {
      var u = ka(e).hoistableStyles, n = Me(l);
      t = t || "default";
      var i = u.get(n);
      if (!i) {
        var c = { loading: 0, preload: null };
        if (i = e.querySelector(
          ru(n)
        ))
          c.loading = 5;
        else {
          l = q(
            { rel: "stylesheet", href: l, "data-precedence": t },
            a
          ), (a = Dt.get(n)) && lf(l, a);
          var f = i = e.createElement("link");
          Yl(f), Kl(f, "link", l), f._p = new Promise(function(h, S) {
            f.onload = h, f.onerror = S;
          }), f.addEventListener("load", function() {
            c.loading |= 1;
          }), f.addEventListener("error", function() {
            c.loading |= 2;
          }), c.loading |= 4, Hn(i, t, e);
        }
        i = {
          type: "stylesheet",
          instance: i,
          count: 1,
          state: c
        }, u.set(n, i);
      }
    }
  }
  function Qy(l, t) {
    la.X(l, t);
    var a = pe;
    if (a && l) {
      var e = ka(a).hoistableScripts, u = Oe(l), n = e.get(u);
      n || (n = a.querySelector(gu(u)), n || (l = q({ src: l, async: !0 }, t), (t = Dt.get(u)) && tf(l, t), n = a.createElement("script"), Yl(n), Kl(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, e.set(u, n));
    }
  }
  function Zy(l, t) {
    la.M(l, t);
    var a = pe;
    if (a && l) {
      var e = ka(a).hoistableScripts, u = Oe(l), n = e.get(u);
      n || (n = a.querySelector(gu(u)), n || (l = q({ src: l, async: !0, type: "module" }, t), (t = Dt.get(u)) && tf(l, t), n = a.createElement("script"), Yl(n), Kl(n, "link", l), a.head.appendChild(n)), n = {
        type: "script",
        instance: n,
        count: 1,
        state: null
      }, e.set(u, n));
    }
  }
  function lv(l, t, a, e) {
    var u = (u = V.current) ? jn(u) : null;
    if (!u) throw Error(s(446));
    switch (l) {
      case "meta":
      case "title":
        return null;
      case "style":
        return typeof a.precedence == "string" && typeof a.href == "string" ? (t = Me(a.href), a = ka(
          u
        ).hoistableStyles, e = a.get(t), e || (e = {
          type: "style",
          instance: null,
          count: 0,
          state: null
        }, a.set(t, e)), e) : { type: "void", instance: null, count: 0, state: null };
      case "link":
        if (a.rel === "stylesheet" && typeof a.href == "string" && typeof a.precedence == "string") {
          l = Me(a.href);
          var n = ka(
            u
          ).hoistableStyles, i = n.get(l);
          if (i || (u = u.ownerDocument || u, i = {
            type: "stylesheet",
            instance: null,
            count: 0,
            state: { loading: 0, preload: null }
          }, n.set(l, i), (n = u.querySelector(
            ru(l)
          )) && !n._p && (i.instance = n, i.state.loading = 5), Dt.has(l) || (a = {
            rel: "preload",
            as: "style",
            href: a.href,
            crossOrigin: a.crossOrigin,
            integrity: a.integrity,
            media: a.media,
            hrefLang: a.hrefLang,
            referrerPolicy: a.referrerPolicy
          }, Dt.set(l, a), n || Ly(
            u,
            l,
            a,
            i.state
          ))), t && e === null)
            throw Error(s(528, ""));
          return i;
        }
        if (t && e !== null)
          throw Error(s(529, ""));
        return null;
      case "script":
        return t = a.async, a = a.src, typeof a == "string" && t && typeof t != "function" && typeof t != "symbol" ? (t = Oe(a), a = ka(
          u
        ).hoistableScripts, e = a.get(t), e || (e = {
          type: "script",
          instance: null,
          count: 0,
          state: null
        }, a.set(t, e)), e) : { type: "void", instance: null, count: 0, state: null };
      default:
        throw Error(s(444, l));
    }
  }
  function Me(l) {
    return 'href="' + _t(l) + '"';
  }
  function ru(l) {
    return 'link[rel="stylesheet"][' + l + "]";
  }
  function tv(l) {
    return q({}, l, {
      "data-precedence": l.precedence,
      precedence: null
    });
  }
  function Ly(l, t, a, e) {
    l.querySelector('link[rel="preload"][as="style"][' + t + "]") ? e.loading = 1 : (t = l.createElement("link"), e.preload = t, t.addEventListener("load", function() {
      return e.loading |= 1;
    }), t.addEventListener("error", function() {
      return e.loading |= 2;
    }), Kl(t, "link", a), Yl(t), l.head.appendChild(t));
  }
  function Oe(l) {
    return '[src="' + _t(l) + '"]';
  }
  function gu(l) {
    return "script[async]" + l;
  }
  function av(l, t, a) {
    if (t.count++, t.instance === null)
      switch (t.type) {
        case "style":
          var e = l.querySelector(
            'style[data-href~="' + _t(a.href) + '"]'
          );
          if (e)
            return t.instance = e, Yl(e), e;
          var u = q({}, a, {
            "data-href": a.href,
            "data-precedence": a.precedence,
            href: null,
            precedence: null
          });
          return e = (l.ownerDocument || l).createElement(
            "style"
          ), Yl(e), Kl(e, "style", u), Hn(e, a.precedence, l), t.instance = e;
        case "stylesheet":
          u = Me(a.href);
          var n = l.querySelector(
            ru(u)
          );
          if (n)
            return t.state.loading |= 4, t.instance = n, Yl(n), n;
          e = tv(a), (u = Dt.get(u)) && lf(e, u), n = (l.ownerDocument || l).createElement("link"), Yl(n);
          var i = n;
          return i._p = new Promise(function(c, f) {
            i.onload = c, i.onerror = f;
          }), Kl(n, "link", e), t.state.loading |= 4, Hn(n, a.precedence, l), t.instance = n;
        case "script":
          return n = Oe(a.src), (u = l.querySelector(
            gu(n)
          )) ? (t.instance = u, Yl(u), u) : (e = a, (u = Dt.get(n)) && (e = q({}, a), tf(e, u)), l = l.ownerDocument || l, u = l.createElement("script"), Yl(u), Kl(u, "link", e), l.head.appendChild(u), t.instance = u);
        case "void":
          return null;
        default:
          throw Error(s(443, t.type));
      }
    else
      t.type === "stylesheet" && (t.state.loading & 4) === 0 && (e = t.instance, t.state.loading |= 4, Hn(e, a.precedence, l));
    return t.instance;
  }
  function Hn(l, t, a) {
    for (var e = a.querySelectorAll(
      'link[rel="stylesheet"][data-precedence],style[data-precedence]'
    ), u = e.length ? e[e.length - 1] : null, n = u, i = 0; i < e.length; i++) {
      var c = e[i];
      if (c.dataset.precedence === t) n = c;
      else if (n !== u) break;
    }
    n ? n.parentNode.insertBefore(l, n.nextSibling) : (t = a.nodeType === 9 ? a.head : a, t.insertBefore(l, t.firstChild));
  }
  function lf(l, t) {
    l.crossOrigin == null && (l.crossOrigin = t.crossOrigin), l.referrerPolicy == null && (l.referrerPolicy = t.referrerPolicy), l.title == null && (l.title = t.title);
  }
  function tf(l, t) {
    l.crossOrigin == null && (l.crossOrigin = t.crossOrigin), l.referrerPolicy == null && (l.referrerPolicy = t.referrerPolicy), l.integrity == null && (l.integrity = t.integrity);
  }
  var Rn = null;
  function ev(l, t, a) {
    if (Rn === null) {
      var e = /* @__PURE__ */ new Map(), u = Rn = /* @__PURE__ */ new Map();
      u.set(a, e);
    } else
      u = Rn, e = u.get(a), e || (e = /* @__PURE__ */ new Map(), u.set(a, e));
    if (e.has(l)) return e;
    for (e.set(l, null), a = a.getElementsByTagName(l), u = 0; u < a.length; u++) {
      var n = a[u];
      if (!(n[Re] || n[Ql] || l === "link" && n.getAttribute("rel") === "stylesheet") && n.namespaceURI !== "http://www.w3.org/2000/svg") {
        var i = n.getAttribute(t) || "";
        i = l + i;
        var c = e.get(i);
        c ? c.push(n) : e.set(i, [n]);
      }
    }
    return e;
  }
  function uv(l, t, a) {
    l = l.ownerDocument || l, l.head.insertBefore(
      a,
      t === "title" ? l.querySelector("head > title") : null
    );
  }
  function Vy(l, t, a) {
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
  function nv(l) {
    return !(l.type === "stylesheet" && (l.state.loading & 3) === 0);
  }
  function Ky(l, t, a, e) {
    if (a.type === "stylesheet" && (typeof e.media != "string" || matchMedia(e.media).matches !== !1) && (a.state.loading & 4) === 0) {
      if (a.instance === null) {
        var u = Me(e.href), n = t.querySelector(
          ru(u)
        );
        if (n) {
          t = n._p, t !== null && typeof t == "object" && typeof t.then == "function" && (l.count++, l = xn.bind(l), t.then(l, l)), a.state.loading |= 4, a.instance = n, Yl(n);
          return;
        }
        n = t.ownerDocument || t, e = tv(e), (u = Dt.get(u)) && lf(e, u), n = n.createElement("link"), Yl(n);
        var i = n;
        i._p = new Promise(function(c, f) {
          i.onload = c, i.onerror = f;
        }), Kl(n, "link", e), a.instance = n;
      }
      l.stylesheets === null && (l.stylesheets = /* @__PURE__ */ new Map()), l.stylesheets.set(a, t), (t = a.state.preload) && (a.state.loading & 3) === 0 && (l.count++, a = xn.bind(l), t.addEventListener("load", a), t.addEventListener("error", a));
    }
  }
  var af = 0;
  function Jy(l, t) {
    return l.stylesheets && l.count === 0 && Bn(l, l.stylesheets), 0 < l.count || 0 < l.imgCount ? function(a) {
      var e = setTimeout(function() {
        if (l.stylesheets && Bn(l, l.stylesheets), l.unsuspend) {
          var n = l.unsuspend;
          l.unsuspend = null, n();
        }
      }, 6e4 + t);
      0 < l.imgBytes && af === 0 && (af = 62500 * My());
      var u = setTimeout(
        function() {
          if (l.waitingForImages = !1, l.count === 0 && (l.stylesheets && Bn(l, l.stylesheets), l.unsuspend)) {
            var n = l.unsuspend;
            l.unsuspend = null, n();
          }
        },
        (l.imgBytes > af ? 50 : 800) + t
      );
      return l.unsuspend = a, function() {
        l.unsuspend = null, clearTimeout(e), clearTimeout(u);
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
    l.stylesheets = null, l.unsuspend !== null && (l.count++, qn = /* @__PURE__ */ new Map(), t.forEach(wy, l), qn = null, xn.call(l));
  }
  function wy(l, t) {
    if (!(t.state.loading & 4)) {
      var a = qn.get(l);
      if (a) var e = a.get(null);
      else {
        a = /* @__PURE__ */ new Map(), qn.set(l, a);
        for (var u = l.querySelectorAll(
          "link[data-precedence],style[data-precedence]"
        ), n = 0; n < u.length; n++) {
          var i = u[n];
          (i.nodeName === "LINK" || i.getAttribute("media") !== "not all") && (a.set(i.dataset.precedence, i), e = i);
        }
        e && a.set(null, e);
      }
      u = t.instance, i = u.getAttribute("data-precedence"), n = a.get(i) || e, n === e && a.set(null, u), a.set(i, u), this.count++, e = xn.bind(this), u.addEventListener("load", e), u.addEventListener("error", e), n ? n.parentNode.insertBefore(u, n.nextSibling) : (l = l.nodeType === 9 ? l.head : l, l.insertBefore(u, l.firstChild)), t.state.loading |= 4;
    }
  }
  var Su = {
    $$typeof: rl,
    Provider: null,
    Consumer: null,
    _currentValue: R,
    _currentValue2: R,
    _threadCount: 0
  };
  function ky(l, t, a, e, u, n, i, c, f) {
    this.tag = 1, this.containerInfo = l, this.pingCache = this.current = this.pendingChildren = null, this.timeoutHandle = -1, this.callbackNode = this.next = this.pendingContext = this.context = this.cancelPendingCommit = null, this.callbackPriority = 0, this.expirationTimes = Fn(-1), this.entangledLanes = this.shellSuspendCounter = this.errorRecoveryDisabledLanes = this.expiredLanes = this.warmLanes = this.pingedLanes = this.suspendedLanes = this.pendingLanes = 0, this.entanglements = Fn(0), this.hiddenUpdates = Fn(null), this.identifierPrefix = e, this.onUncaughtError = u, this.onCaughtError = n, this.onRecoverableError = i, this.pooledCache = null, this.pooledCacheLanes = 0, this.formState = f, this.incompleteTransitions = /* @__PURE__ */ new Map();
  }
  function iv(l, t, a, e, u, n, i, c, f, h, S, E) {
    return l = new ky(
      l,
      t,
      a,
      i,
      f,
      h,
      S,
      E,
      c
    ), t = 1, n === !0 && (t |= 24), n = dt(3, null, null, t), l.current = n, n.stateNode = l, t = qi(), t.refCount++, l.pooledCache = t, t.refCount++, n.memoizedState = {
      element: e,
      isDehydrated: a,
      cache: t
    }, Gi(n), l;
  }
  function cv(l) {
    return l ? (l = ue, l) : ue;
  }
  function fv(l, t, a, e, u, n) {
    u = cv(u), e.context === null ? e.context = u : e.pendingContext = u, e = sa(t), e.payload = { element: a }, n = n === void 0 ? null : n, n !== null && (e.callback = n), a = oa(l, e, t), a !== null && (it(a, l, t), $e(a, l, t));
  }
  function sv(l, t) {
    if (l = l.memoizedState, l !== null && l.dehydrated !== null) {
      var a = l.retryLane;
      l.retryLane = a !== 0 && a < t ? a : t;
    }
  }
  function ef(l, t) {
    sv(l, t), (l = l.alternate) && sv(l, t);
  }
  function ov(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = Ha(l, 67108864);
      t !== null && it(t, l, 67108864), ef(l, 67108864);
    }
  }
  function dv(l) {
    if (l.tag === 13 || l.tag === 31) {
      var t = rt();
      t = In(t);
      var a = Ha(l, t);
      a !== null && it(a, l, t), ef(l, t);
    }
  }
  var Cn = !0;
  function Wy(l, t, a, e) {
    var u = b.T;
    b.T = null;
    var n = _.p;
    try {
      _.p = 2, uf(l, t, a, e);
    } finally {
      _.p = n, b.T = u;
    }
  }
  function $y(l, t, a, e) {
    var u = b.T;
    b.T = null;
    var n = _.p;
    try {
      _.p = 8, uf(l, t, a, e);
    } finally {
      _.p = n, b.T = u;
    }
  }
  function uf(l, t, a, e) {
    if (Cn) {
      var u = nf(e);
      if (u === null)
        Vc(
          l,
          t,
          e,
          Yn,
          a
        ), yv(l, e);
      else if (Iy(
        u,
        l,
        t,
        a,
        e
      ))
        e.stopPropagation();
      else if (yv(l, e), t & 4 && -1 < Fy.indexOf(l)) {
        for (; u !== null; ) {
          var n = wa(u);
          if (n !== null)
            switch (n.tag) {
              case 3:
                if (n = n.stateNode, n.current.memoizedState.isDehydrated) {
                  var i = Oa(n.pendingLanes);
                  if (i !== 0) {
                    var c = n;
                    for (c.pendingLanes |= 2, c.entangledLanes |= 2; i; ) {
                      var f = 1 << 31 - st(i);
                      c.entanglements[1] |= f, i &= ~f;
                    }
                    qt(n), (ul & 6) === 0 && (_n = ct() + 500, vu(0));
                  }
                }
                break;
              case 31:
              case 13:
                c = Ha(n, 2), c !== null && it(c, n, 2), En(), ef(n, 2);
            }
          if (n = nf(e), n === null && Vc(
            l,
            t,
            e,
            Yn,
            a
          ), n === u) break;
          u = n;
        }
        u !== null && e.stopPropagation();
      } else
        Vc(
          l,
          t,
          e,
          null,
          a
        );
    }
  }
  function nf(l) {
    return l = ci(l), cf(l);
  }
  var Yn = null;
  function cf(l) {
    if (Yn = null, l = Ja(l), l !== null) {
      var t = W(l);
      if (t === null) l = null;
      else {
        var a = t.tag;
        if (a === 13) {
          if (l = P(t), l !== null) return l;
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
  function vv(l) {
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
        switch (Bv()) {
          case bf:
            return 2;
          case _f:
            return 8;
          case Mu:
          case Cv:
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
  var ff = !1, za = null, Ea = null, Ta = null, bu = /* @__PURE__ */ new Map(), _u = /* @__PURE__ */ new Map(), Aa = [], Fy = "mousedown mouseup touchcancel touchend touchstart auxclick dblclick pointercancel pointerdown pointerup dragend dragstart drop compositionend compositionstart keydown keypress keyup input textInput copy cut paste click change contextmenu reset".split(
    " "
  );
  function yv(l, t) {
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
        bu.delete(t.pointerId);
        break;
      case "gotpointercapture":
      case "lostpointercapture":
        _u.delete(t.pointerId);
    }
  }
  function zu(l, t, a, e, u, n) {
    return l === null || l.nativeEvent !== n ? (l = {
      blockedOn: t,
      domEventName: a,
      eventSystemFlags: e,
      nativeEvent: n,
      targetContainers: [u]
    }, t !== null && (t = wa(t), t !== null && ov(t)), l) : (l.eventSystemFlags |= e, t = l.targetContainers, u !== null && t.indexOf(u) === -1 && t.push(u), l);
  }
  function Iy(l, t, a, e, u) {
    switch (t) {
      case "focusin":
        return za = zu(
          za,
          l,
          t,
          a,
          e,
          u
        ), !0;
      case "dragenter":
        return Ea = zu(
          Ea,
          l,
          t,
          a,
          e,
          u
        ), !0;
      case "mouseover":
        return Ta = zu(
          Ta,
          l,
          t,
          a,
          e,
          u
        ), !0;
      case "pointerover":
        var n = u.pointerId;
        return bu.set(
          n,
          zu(
            bu.get(n) || null,
            l,
            t,
            a,
            e,
            u
          )
        ), !0;
      case "gotpointercapture":
        return n = u.pointerId, _u.set(
          n,
          zu(
            _u.get(n) || null,
            l,
            t,
            a,
            e,
            u
          )
        ), !0;
    }
    return !1;
  }
  function mv(l) {
    var t = Ja(l.target);
    if (t !== null) {
      var a = W(t);
      if (a !== null) {
        if (t = a.tag, t === 13) {
          if (t = P(a), t !== null) {
            l.blockedOn = t, Of(l.priority, function() {
              dv(a);
            });
            return;
          }
        } else if (t === 31) {
          if (t = gl(a), t !== null) {
            l.blockedOn = t, Of(l.priority, function() {
              dv(a);
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
      var a = nf(l.nativeEvent);
      if (a === null) {
        a = l.nativeEvent;
        var e = new a.constructor(
          a.type,
          a
        );
        ii = e, a.target.dispatchEvent(e), ii = null;
      } else
        return t = wa(a), t !== null && ov(t), l.blockedOn = a, !1;
      t.shift();
    }
    return !0;
  }
  function hv(l, t, a) {
    Gn(l) && a.delete(t);
  }
  function Py() {
    ff = !1, za !== null && Gn(za) && (za = null), Ea !== null && Gn(Ea) && (Ea = null), Ta !== null && Gn(Ta) && (Ta = null), bu.forEach(hv), _u.forEach(hv);
  }
  function Xn(l, t) {
    l.blockedOn === t && (l.blockedOn = null, ff || (ff = !0, v.unstable_scheduleCallback(
      v.unstable_NormalPriority,
      Py
    )));
  }
  var Qn = null;
  function rv(l) {
    Qn !== l && (Qn = l, v.unstable_scheduleCallback(
      v.unstable_NormalPriority,
      function() {
        Qn === l && (Qn = null);
        for (var t = 0; t < l.length; t += 3) {
          var a = l[t], e = l[t + 1], u = l[t + 2];
          if (typeof e != "function") {
            if (cf(e || a) === null)
              continue;
            break;
          }
          var n = wa(a);
          n !== null && (l.splice(t, 3), t -= 3, nc(
            n,
            {
              pending: !0,
              data: u,
              method: a.method,
              action: e
            },
            e,
            u
          ));
        }
      }
    ));
  }
  function De(l) {
    function t(f) {
      return Xn(f, l);
    }
    za !== null && Xn(za, l), Ea !== null && Xn(Ea, l), Ta !== null && Xn(Ta, l), bu.forEach(t), _u.forEach(t);
    for (var a = 0; a < Aa.length; a++) {
      var e = Aa[a];
      e.blockedOn === l && (e.blockedOn = null);
    }
    for (; 0 < Aa.length && (a = Aa[0], a.blockedOn === null); )
      mv(a), a.blockedOn === null && Aa.shift();
    if (a = (l.ownerDocument || l).$$reactFormReplay, a != null)
      for (e = 0; e < a.length; e += 3) {
        var u = a[e], n = a[e + 1], i = u[lt] || null;
        if (typeof n == "function")
          i || rv(a);
        else if (i) {
          var c = null;
          if (n && n.hasAttribute("formAction")) {
            if (u = n, i = n[lt] || null)
              c = i.formAction;
            else if (cf(u) !== null) continue;
          } else c = i.action;
          typeof c == "function" ? a[e + 1] = c : (a.splice(e, 3), e -= 3), rv(a);
        }
      }
  }
  function gv() {
    function l(n) {
      n.canIntercept && n.info === "react-transition" && n.intercept({
        handler: function() {
          return new Promise(function(i) {
            return u = i;
          });
        },
        focusReset: "manual",
        scroll: "manual"
      });
    }
    function t() {
      u !== null && (u(), u = null), e || setTimeout(a, 20);
    }
    function a() {
      if (!e && !navigation.transition) {
        var n = navigation.currentEntry;
        n && n.url != null && navigation.navigate(n.url, {
          state: n.getState(),
          info: "react-transition",
          history: "replace"
        });
      }
    }
    if (typeof navigation == "object") {
      var e = !1, u = null;
      return navigation.addEventListener("navigate", l), navigation.addEventListener("navigatesuccess", t), navigation.addEventListener("navigateerror", t), setTimeout(a, 100), function() {
        e = !0, navigation.removeEventListener("navigate", l), navigation.removeEventListener("navigatesuccess", t), navigation.removeEventListener("navigateerror", t), u !== null && (u(), u = null);
      };
    }
  }
  function sf(l) {
    this._internalRoot = l;
  }
  Zn.prototype.render = sf.prototype.render = function(l) {
    var t = this._internalRoot;
    if (t === null) throw Error(s(409));
    var a = t.current, e = rt();
    fv(a, e, l, t, null, null);
  }, Zn.prototype.unmount = sf.prototype.unmount = function() {
    var l = this._internalRoot;
    if (l !== null) {
      this._internalRoot = null;
      var t = l.containerInfo;
      fv(l.current, 2, null, l, null, null), En(), t[Ka] = null;
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
      Aa.splice(a, 0, l), a === 0 && mv(l);
    }
  };
  var Sv = O.version;
  if (Sv !== "19.2.4")
    throw Error(
      s(
        527,
        Sv,
        "19.2.4"
      )
    );
  _.findDOMNode = function(l) {
    var t = l._reactInternals;
    if (t === void 0)
      throw typeof l.render == "function" ? Error(s(188)) : (l = Object.keys(l).join(","), Error(s(268, l)));
    return l = p(t), l = l !== null ? L(l) : null, l = l === null ? null : l.stateNode, l;
  };
  var lm = {
    bundleType: 0,
    version: "19.2.4",
    rendererPackageName: "react-dom",
    currentDispatcherRef: b,
    reconcilerVersion: "19.2.4"
  };
  if (typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ < "u") {
    var Ln = __REACT_DEVTOOLS_GLOBAL_HOOK__;
    if (!Ln.isDisabled && Ln.supportsFiber)
      try {
        Ne = Ln.inject(
          lm
        ), ft = Ln;
      } catch {
      }
  }
  return Tu.createRoot = function(l, t) {
    if (!x(l)) throw Error(s(299));
    var a = !1, e = "", u = po, n = Mo, i = Oo;
    return t != null && (t.unstable_strictMode === !0 && (a = !0), t.identifierPrefix !== void 0 && (e = t.identifierPrefix), t.onUncaughtError !== void 0 && (u = t.onUncaughtError), t.onCaughtError !== void 0 && (n = t.onCaughtError), t.onRecoverableError !== void 0 && (i = t.onRecoverableError)), t = iv(
      l,
      1,
      !1,
      null,
      null,
      a,
      e,
      null,
      u,
      n,
      i,
      gv
    ), l[Ka] = t.current, Lc(l), new sf(t);
  }, Tu.hydrateRoot = function(l, t, a) {
    if (!x(l)) throw Error(s(299));
    var e = !1, u = "", n = po, i = Mo, c = Oo, f = null;
    return a != null && (a.unstable_strictMode === !0 && (e = !0), a.identifierPrefix !== void 0 && (u = a.identifierPrefix), a.onUncaughtError !== void 0 && (n = a.onUncaughtError), a.onCaughtError !== void 0 && (i = a.onCaughtError), a.onRecoverableError !== void 0 && (c = a.onRecoverableError), a.formState !== void 0 && (f = a.formState)), t = iv(
      l,
      1,
      !0,
      t,
      a ?? null,
      e,
      u,
      f,
      n,
      i,
      c,
      gv
    ), t.context = cv(null), a = t.current, e = rt(), e = In(e), u = sa(e), u.callback = null, oa(a, u, e), a = e, t.current.lanes = a, He(t, a), qt(t), l[Ka] = t.current, Lc(l), new Zn(t);
  }, Tu.version = "19.2.4", Tu;
}
var Dv;
function vm() {
  if (Dv) return vf.exports;
  Dv = 1;
  function v() {
    if (!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ > "u" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE != "function"))
      try {
        __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(v);
      } catch (O) {
        console.error(O);
      }
  }
  return v(), vf.exports = dm(), vf.exports;
}
var ym = vm();
const mm = "_skeleton_xk662_19", hm = "_card_xk662_32", rm = "_row_xk662_40", gm = "_block_xk662_47", Sm = "_textStack_xk662_54", bm = "_textLine_xk662_61", _m = "_textLineLast_xk662_68", Bt = {
  skeleton: mm,
  card: hm,
  row: rm,
  block: gm,
  textStack: Sm,
  textLine: bm,
  textLineLast: _m
};
function zm(v) {
  switch (v.variant) {
    case "card":
      return /* @__PURE__ */ M.jsx(
        "div",
        {
          className: [Bt.skeleton, Bt.card, v.className].filter(Boolean).join(" ")
        }
      );
    case "row":
      return /* @__PURE__ */ M.jsx(
        "div",
        {
          className: [Bt.skeleton, Bt.row, v.className].filter(Boolean).join(" ")
        }
      );
    case "block":
      return /* @__PURE__ */ M.jsx(
        "div",
        {
          className: [Bt.skeleton, Bt.block, v.className].filter(Boolean).join(" ")
        }
      );
    case "text": {
      const O = v.lines ?? 3;
      return /* @__PURE__ */ M.jsx(
        "div",
        {
          className: [Bt.textStack, v.className].filter(Boolean).join(" "),
          children: Array.from({ length: O }, (D, s) => /* @__PURE__ */ M.jsx(
            "div",
            {
              className: [
                Bt.skeleton,
                Bt.textLine,
                s === O - 1 && O > 1 ? Bt.textLineLast : ""
              ].filter(Boolean).join(" ")
            },
            s
          ))
        }
      );
    }
  }
}
const Em = "_badge_vkl6x_1", Tm = "_ready_vkl6x_12", Am = "_planning_vkl6x_18", pm = "_implementing_vkl6x_24", Mm = "_reviewing_vkl6x_30", Om = "_verifying_vkl6x_36", Dm = "_done_vkl6x_42", Um = "_cancelled_vkl6x_48", Nm = "_pending_vkl6x_54", jm = "_running_vkl6x_60", Hm = "_complete_vkl6x_66", Rm = "_failed_vkl6x_72", xm = "_closed_vkl6x_78", qm = "_blocked_vkl6x_84", Bm = "_inReview_vkl6x_90", Cm = "_loading_vkl6x_96", Ym = "_paused_vkl6x_102", Gm = "_unknown_vkl6x_108", Xl = {
  badge: Em,
  ready: Tm,
  planning: Am,
  implementing: pm,
  reviewing: Mm,
  verifying: Om,
  done: Dm,
  cancelled: Um,
  pending: Nm,
  running: jm,
  complete: Hm,
  failed: Rm,
  closed: xm,
  blocked: qm,
  inReview: Bm,
  loading: Cm,
  paused: Ym,
  unknown: Gm
}, Xm = {
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
}, Qm = {
  ready: Xl.ready,
  planning: Xl.planning,
  implementing: Xl.implementing,
  reviewing: Xl.reviewing,
  verifying: Xl.verifying,
  done: Xl.done,
  cancelled: Xl.cancelled,
  pending: Xl.pending,
  running: Xl.running,
  complete: Xl.complete,
  failed: Xl.failed,
  closed: Xl.closed,
  blocked: Xl.blocked,
  in_review: Xl.inReview,
  loading: Xl.loading,
  paused: Xl.paused
};
function Au({ status: v }) {
  const O = Xm[v] ?? v, D = Qm[v] ?? Xl.unknown;
  return /* @__PURE__ */ M.jsx("span", { className: `${Xl.badge} ${D}`, children: O });
}
const Zm = "_root_ml2s2_1", Lm = "_dark_ml2s2_2", Vm = "_light_ml2s2_3", Km = "_header_ml2s2_4", Jm = "_controls_ml2s2_4", wm = "_lifecycle_ml2s2_4", km = "_pipRail_ml2s2_4", Wm = "_selectors_ml2s2_4", $m = "_task_ml2s2_4", Fm = "_event_ml2s2_4", Im = "_connection_ml2s2_6", Pm = "_meta_ml2s2_6", lh = "_muted_ml2s2_6", th = "_stale_ml2s2_7", ah = "_chip_ml2s2_9", eh = "_list_ml2s2_10", uh = "_card_ml2s2_11", nh = "_attention_ml2s2_11", ih = "_readiness_ml2s2_11", ch = "_waveBoard_ml2s2_13", fh = "_linkButton_ml2s2_25", sh = "_pip_ml2s2_4", oh = "_inlineMode_ml2s2_27", dh = "_fullscreen_ml2s2_27", vh = "_sidebar_ml2s2_28", al = {
  root: Zm,
  dark: Lm,
  light: Vm,
  header: Km,
  controls: Jm,
  lifecycle: wm,
  pipRail: km,
  selectors: Wm,
  task: $m,
  event: Fm,
  connection: Im,
  meta: Pm,
  muted: lh,
  stale: th,
  chip: ah,
  list: eh,
  card: uh,
  attention: nh,
  readiness: ih,
  waveBoard: ch,
  linkButton: fh,
  pip: sh,
  inlineMode: oh,
  fullscreen: dh,
  sidebar: vh
};
function yh({ agents: v }) {
  return /* @__PURE__ */ M.jsxs("section", { children: [
    /* @__PURE__ */ M.jsx("h2", { children: "agents" }),
    /* @__PURE__ */ M.jsx("ul", { className: al.list, "aria-label": "active agents", children: v.map((O, D) => /* @__PURE__ */ M.jsxs("li", { className: al.card, children: [
      /* @__PURE__ */ M.jsxs("div", { children: [
        /* @__PURE__ */ M.jsx("strong", { children: O.role }),
        " · ",
        O.task,
        O.wave ? ` · wave ${O.wave}` : "",
        O.task_number ? ` task ${O.task_number}` : ""
      ] }),
      /* @__PURE__ */ M.jsxs("div", { className: al.meta, children: [
        O.branch || "no branch",
        " · ",
        O.worktree || "no worktree",
        " · ",
        O.last_activity ? new Date(O.last_activity).toLocaleTimeString() : "no activity"
      ] }),
      /* @__PURE__ */ M.jsx(Au, { status: O.paused ? "paused" : O.stage || (O.active ? "running" : "ready") })
    ] }, `${O.task}-${O.role}-${D}`)) })
  ] });
}
function mh({ items: v, action: O }) {
  return /* @__PURE__ */ M.jsxs("section", { children: [
    /* @__PURE__ */ M.jsx("h2", { children: "attention" }),
    v.length === 0 ? /* @__PURE__ */ M.jsx("p", { className: al.muted, children: "nothing needs attention" }) : /* @__PURE__ */ M.jsx("ul", { className: al.list, "aria-label": "attention items", children: v.map((D, s) => /* @__PURE__ */ M.jsxs("li", { className: al.attention, children: [
      /* @__PURE__ */ M.jsxs("div", { children: [
        /* @__PURE__ */ M.jsx("strong", { children: D.kind.replace(/_/g, " ") }),
        " · ",
        D.task
      ] }),
      D.detail && /* @__PURE__ */ M.jsx("p", { children: D.detail }),
      O && /* @__PURE__ */ M.jsx("button", { onClick: () => O(`look at the blocker on ${D.task}`), children: "look at blocker" })
    ] }, `${D.task}-${D.kind}-${s}`)) })
  ] });
}
function hh({ events: v }) {
  return /* @__PURE__ */ M.jsxs("section", { children: [
    /* @__PURE__ */ M.jsx("h2", { children: "events" }),
    /* @__PURE__ */ M.jsx("ol", { className: al.list, "aria-label": "event feed", children: v.map((O, D) => /* @__PURE__ */ M.jsxs("li", { className: al.event, children: [
      /* @__PURE__ */ M.jsx("time", { dateTime: O.at, children: new Date(O.at).toLocaleTimeString() }),
      /* @__PURE__ */ M.jsx("span", { children: O.message })
    ] }, `${O.at}-${D}`)) })
  ] });
}
function Uv({ lifecycle: v }) {
  const O = ["planning", "ready", "implementing", "reviewing", "verifying"];
  return /* @__PURE__ */ M.jsx("div", { className: al.lifecycle, role: "list", "aria-label": "lifecycle counts", children: O.map((D) => /* @__PURE__ */ M.jsxs("span", { role: "listitem", className: al.chip, children: [
    /* @__PURE__ */ M.jsx("b", { children: v[D] }),
    " ",
    D
  ] }, D)) });
}
function rh(v, O) {
  const D = (v == null ? void 0 : v.active_agents.filter((s) => s.active && !s.paused).length) ?? 0;
  return {
    level: !v || !v.daemon_running ? "offline" : v.attention.length ? "attention" : D ? "running" : "idle",
    running_agents: D,
    blocked: (v == null ? void 0 : v.attention.length) ?? 0,
    implementing: (v == null ? void 0 : v.lifecycle.implementing) ?? 0,
    reviewing: (v == null ? void 0 : v.lifecycle.reviewing) ?? 0,
    project: O.project,
    task: O.task
  };
}
function gh() {
  return window.openai ?? {};
}
function Sh() {
  const [v, O] = I.useState(gh);
  return I.useEffect(() => {
    const D = (s) => {
      const x = s.detail, W = (x == null ? void 0 : x.globals) ?? x ?? {};
      O((P) => ({ ...P, ...W }));
    };
    return window.addEventListener("openai:set_globals", D), () => window.removeEventListener("openai:set_globals", D);
  }, []), v;
}
function jv(v) {
  if (!v || typeof v != "object") return;
  const O = v, D = O.structuredContent ?? O.structured_content ?? v;
  if (!D || typeof D != "object") return;
  const s = D;
  if (!(s.schema_version !== 2 || typeof s.project != "string" || typeof s.daemon_running != "boolean" || !s.lifecycle || typeof s.lifecycle != "object" || !Array.isArray(s.active_agents) || !Array.isArray(s.attention) || !s.truncated || typeof s.truncated != "object"))
    return s;
}
function bh(v) {
  const O = v, D = (O == null ? void 0 : O.structuredContent) ?? (O == null ? void 0 : O.structured_content) ?? v, s = D && typeof D == "object" ? D.schema_version : void 0;
  return typeof s == "number" && s !== Eh ? "incompatible-schema" : "invalid";
}
function _h(v) {
  const O = {
    contractVersion: Vn,
    snapshot: jv(v.toolOutput),
    input: v.toolInput,
    state: v.widgetState,
    displayMode: v.displayMode ?? "inline",
    visibility: document.visibilityState === "visible" ? "expanded" : "hidden",
    theme: v.theme ?? "dark",
    maxHeight: v.maxHeight,
    refresh: async (D) => {
      var x, W;
      return await ((W = (x = window.openai) == null ? void 0 : x.callTool) == null ? void 0 : W.call(x, "refresh_monitor", { project: D.project, task: D.task }));
    },
    subscribe: (D) => (window.addEventListener("openai:set_globals", D), () => window.removeEventListener("openai:set_globals", D))
  };
  return v.setWidgetState && (O.saveState = (D) => {
    var s, x;
    return (x = (s = window.openai) == null ? void 0 : s.setWidgetState) == null ? void 0 : x.call(s, D);
  }), v.requestDisplayMode && (O.requestDisplayMode = (D) => {
    var s, x;
    return (x = (s = window.openai) == null ? void 0 : s.requestDisplayMode) == null ? void 0 : x.call(s, { mode: D });
  }), v.sendFollowUpMessage && (O.sendPrompt = (D) => {
    var s, x;
    return (x = (s = window.openai) == null ? void 0 : s.sendFollowUpMessage) == null ? void 0 : x.call(s, { prompt: D });
  }), O;
}
function zh(v, O, D) {
  const [s, x] = I.useState(v.snapshot), [W, P] = I.useState(v.contractVersion === Vn ? v.snapshot ? "ready" : "loading" : "incompatible"), [gl, A] = I.useState(!1), p = I.useRef(s);
  p.current = s;
  const L = I.useRef(W);
  L.current = W;
  const q = `${O ?? ""}\0${D ?? ""}`, el = I.useRef(q);
  el.current = q;
  const Sl = I.useRef(!1), zl = I.useRef(!1), Ml = I.useRef(async () => {
  }), Wl = I.useRef(!1), Ol = I.useRef(0), $l = I.useRef(void 0), rl = I.useRef(!0), El = document.visibilityState !== "visible" ? "hidden" : v.visibility, Jl = I.useRef(El), Dl = El === "collapsed" ? 15e3 : v.displayMode === "inline" ? 3e3 : 2e3;
  I.useEffect(() => {
    v.snapshot && (x(v.snapshot), P("ready"));
  }, [v.snapshot]), I.useEffect(() => () => {
    rl.current = !1;
  }, []);
  const Y = I.useCallback(async () => {
    const Rl = q;
    if (Sl.current) {
      zl.current = !0;
      return;
    }
    if (!(document.visibilityState !== "visible" || v.visibility === "hidden" || L.current === "incompatible")) {
      Sl.current = !0;
      try {
        const wl = await v.refresh({ project: O, task: D }), xl = jv(wl);
        if (!xl) {
          if (!p.current && bh(wl) === "incompatible-schema") {
            P("incompatible");
            return;
          }
          throw new Error("invalid snapshot");
        }
        rl.current && el.current === Rl && (x(xl), Ol.current = 0, A(!1), P("ready"));
      } catch {
        rl.current && el.current === Rl && (Ol.current += 1, A(!0), p.current || P("offline"));
      } finally {
        Sl.current = !1, zl.current && (zl.current = !1, Ml.current());
      }
    }
  }, [v, O, q, D]);
  return Ml.current = Y, I.useEffect(() => {
    !Wl.current && !v.snapshot && W === "loading" && (Wl.current = !0, Y());
  }, [v.snapshot, W, Y]), I.useEffect(() => {
    let Rl = !1;
    const wl = () => {
      if (window.clearTimeout($l.current), Rl || document.visibilityState !== "visible" || v.visibility === "hidden" || W === "incompatible") return;
      const gt = Math.min(Dl * 2 ** Ol.current, 3e4);
      $l.current = window.setTimeout(async () => {
        await Y(), wl();
      }, gt);
    }, xl = () => {
      document.visibilityState === "visible" && v.visibility !== "hidden" && Y(), wl();
    }, ql = Jl.current !== "expanded" && El === "expanded";
    return Jl.current = El, ql && Y(), wl(), document.addEventListener("visibilitychange", xl), () => {
      Rl = !0, window.clearTimeout($l.current), document.removeEventListener("visibilitychange", xl);
    };
  }, [Dl, El, v.visibility, W, Y]), { snapshot: !O || (s == null ? void 0 : s.project) === O ? s : void 0, stale: gl, phase: W, refresh: Y };
}
const Vn = 1, Eh = 2;
function Th(v) {
  return v.contractVersion !== Vn ? Hv() : v;
}
function Hv() {
  return { contractVersion: -1, displayMode: "inline", visibility: "expanded", theme: "dark", refresh: async () => {
    throw new Error("monitor host contract version mismatch");
  }, subscribe: () => () => {
  } };
}
function Ah() {
  return { contractVersion: Vn, displayMode: "inline", visibility: "expanded", theme: "dark", refresh: async () => {
    throw new Error("monitor host unavailable");
  }, subscribe: () => () => {
  } };
}
function ph() {
  const v = I.useRef(null);
  v.current === null && (v.current = window.kasmosMonitorHost ?? Hv());
  const O = Sh(), [, D] = I.useReducer((x) => x + 1, 0), s = window.kasmosMonitorHost;
  return I.useEffect(() => {
    if (s != null && s.subscribe)
      return s.subscribe(() => D());
  }, [s]), I.useMemo(() => s ? Th(s) : window.openai ? _h(O) : Ah(), [s, O]);
}
function Mh({ focus: v, action: O }) {
  return /* @__PURE__ */ M.jsxs("section", { children: [
    /* @__PURE__ */ M.jsx("h2", { children: "waves" }),
    /* @__PURE__ */ M.jsx("div", { className: al.waveBoard, role: "list", children: v.waves.map((D) => /* @__PURE__ */ M.jsxs("article", { role: "listitem", className: al.card, children: [
      /* @__PURE__ */ M.jsxs("header", { children: [
        /* @__PURE__ */ M.jsxs("strong", { children: [
          "wave ",
          D.wave
        ] }),
        D.active && /* @__PURE__ */ M.jsx("span", { children: " active" })
      ] }),
      /* @__PURE__ */ M.jsx("ul", { children: D.tasks.map((s) => /* @__PURE__ */ M.jsxs("li", { children: [
        /* @__PURE__ */ M.jsxs("span", { children: [
          s.number,
          ". ",
          s.title
        ] }),
        /* @__PURE__ */ M.jsx(Au, { status: s.status })
      ] }, s.number)) }),
      D.active && O && /* @__PURE__ */ M.jsxs("button", { onClick: () => O(`start wave ${D.wave} on ${v.filename}`), children: [
        "start wave ",
        D.wave
      ] })
    ] }, D.wave)) })
  ] });
}
function Oh() {
  var Wl, Ol, $l, rl, El, Jl, Dl, Y, Cl, Rl, wl, xl, ql, gt, St, Pl, b;
  const v = ph(), O = v.displayMode, D = ((Wl = v.state) == null ? void 0 : Wl.project) ?? ((Ol = v.input) == null ? void 0 : Ol.project) ?? (($l = v.snapshot) == null ? void 0 : $l.project) ?? ((El = (rl = v.snapshot) == null ? void 0 : rl.projects) == null ? void 0 : El[0]), s = ((Jl = v.state) == null ? void 0 : Jl.task) ?? ((Dl = v.input) == null ? void 0 : Dl.task) ?? ((Cl = (Y = v.snapshot) == null ? void 0 : Y.focus) == null ? void 0 : Cl.filename) ?? ((xl = (wl = (Rl = v.snapshot) == null ? void 0 : Rl.tasks) == null ? void 0 : wl[0]) == null ? void 0 : xl.filename), [x, W] = I.useState(D), [P, gl] = I.useState(s), { snapshot: A, stale: p, phase: L, refresh: q } = zh(v, x, P);
  I.useEffect(() => {
    var _;
    !x && A && W(A.project || ((_ = A.projects) == null ? void 0 : _[0]));
  }, [x, A]), I.useEffect(() => {
    var _, R, F;
    !P && A && A.project === x && gl(((_ = A.focus) == null ? void 0 : _.filename) ?? ((F = (R = A.tasks) == null ? void 0 : R[0]) == null ? void 0 : F.filename));
  }, [x, P, A]), I.useEffect(() => {
    var _;
    (_ = v.saveState) == null || _.call(v, { project: x, task: P });
  }, [v.saveState, x, P]);
  const el = I.useRef(void 0);
  I.useEffect(() => {
    var F, nl;
    if (!A && L === "loading") return;
    const _ = rh(A, { project: x ?? (A == null ? void 0 : A.project), task: P ?? ((F = A == null ? void 0 : A.focus) == null ? void 0 : F.filename) }), R = JSON.stringify(_);
    R !== el.current && (el.current = R, (nl = v.setBadge) == null || nl.call(v, _));
  }, [v.setBadge, L, x, A, P]);
  const Sl = v.sendPrompt ? (_) => {
    var R;
    (R = v.sendPrompt) == null || R.call(v, _);
  } : void 0, zl = (A == null ? void 0 : A.attention.length) ?? 0, Ml = I.useMemo(() => (A == null ? void 0 : A.active_agents.filter((_) => _.active && !_.paused).length) ?? 0, [A]);
  return !A && L === "loading" ? /* @__PURE__ */ M.jsx("main", { className: al.root, children: /* @__PURE__ */ M.jsx(zm, { variant: "text", lines: 4 }) }) : !A && L === "incompatible" ? /* @__PURE__ */ M.jsx("main", { className: al.root, children: "monitor bundle / host version mismatch" }) : A ? /* @__PURE__ */ M.jsxs("main", { className: `${al.root} ${v.theme === "light" ? al.light : al.dark} ${O === "pip" ? al.pip : O === "fullscreen" ? al.fullscreen : O === "sidebar" ? al.sidebar : al.inlineMode}`, style: O === "inline" && v.maxHeight ? { maxHeight: v.maxHeight } : void 0, children: [
    /* @__PURE__ */ M.jsxs("header", { className: al.header, children: [
      /* @__PURE__ */ M.jsxs("div", { children: [
        /* @__PURE__ */ M.jsx("strong", { children: "kasmos monitor" }),
        /* @__PURE__ */ M.jsx("span", { className: al.connection, children: A.daemon_running ? "● live" : "○ daemon offline" })
      ] }),
      /* @__PURE__ */ M.jsxs("div", { className: al.controls, children: [
        v.requestDisplayMode && O !== "sidebar" && O !== "pip" && /* @__PURE__ */ M.jsx("button", { "aria-label": "pin as picture in picture", onClick: () => {
          var _;
          return void ((_ = v.requestDisplayMode) == null ? void 0 : _.call(v, "pip"));
        }, children: "pin" }),
        v.requestDisplayMode && O !== "sidebar" && O !== "fullscreen" && /* @__PURE__ */ M.jsx("button", { "aria-label": "expand monitor", onClick: () => {
          var _;
          return void ((_ = v.requestDisplayMode) == null ? void 0 : _.call(v, "fullscreen"));
        }, children: "expand" })
      ] })
    ] }),
    /* @__PURE__ */ M.jsx("div", { className: al.stale, "aria-live": "polite", children: p ? "stale · retrying with last known state" : "" }),
    O === "pip" ? /* @__PURE__ */ M.jsxs("div", { className: al.pipRail, children: [
      /* @__PURE__ */ M.jsx(Uv, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ M.jsxs("span", { children: [
        Ml,
        " running"
      ] }),
      /* @__PURE__ */ M.jsx(Au, { status: zl ? `${zl} blocked` : "ready" })
    ] }) : /* @__PURE__ */ M.jsxs(M.Fragment, { children: [
      /* @__PURE__ */ M.jsx(Uv, { lifecycle: A.lifecycle }),
      /* @__PURE__ */ M.jsxs("div", { className: al.selectors, children: [
        (((ql = A.projects) == null ? void 0 : ql.length) ?? 0) > 1 && /* @__PURE__ */ M.jsxs("label", { children: [
          "project",
          /* @__PURE__ */ M.jsx("select", { value: x, onChange: (_) => {
            W(_.target.value), gl(void 0);
          }, children: (gt = A.projects) == null ? void 0 : gt.map((_) => /* @__PURE__ */ M.jsx("option", { children: _ }, _)) })
        ] }),
        (((St = A.tasks) == null ? void 0 : St.length) ?? 0) > 0 && /* @__PURE__ */ M.jsxs("label", { children: [
          "task",
          /* @__PURE__ */ M.jsx("select", { value: P, onChange: (_) => gl(_.target.value), children: (Pl = A.tasks) == null ? void 0 : Pl.map((_) => /* @__PURE__ */ M.jsx("option", { value: _.filename, children: _.filename }, _.filename)) })
        ] })
      ] }),
      /* @__PURE__ */ M.jsxs("section", { children: [
        /* @__PURE__ */ M.jsx("h2", { children: "tasks" }),
        /* @__PURE__ */ M.jsx("ul", { className: al.list, "aria-label": "tasks", children: (b = A.tasks) == null ? void 0 : b.map((_) => {
          const R = _.subtasks_total ? Math.round(_.subtasks_done / _.subtasks_total * 100) : 0;
          return /* @__PURE__ */ M.jsxs("li", { className: al.task, children: [
            /* @__PURE__ */ M.jsxs("div", { children: [
              /* @__PURE__ */ M.jsx("button", { className: al.linkButton, onClick: () => gl(_.filename), children: _.filename }),
              /* @__PURE__ */ M.jsx(Au, { status: _.blocked ? "blocked" : _.status })
            ] }),
            /* @__PURE__ */ M.jsx("progress", { value: _.subtasks_done, max: _.subtasks_total || 1, "aria-label": `${_.filename} progress` }),
            /* @__PURE__ */ M.jsxs("small", { children: [
              R,
              "% · wave ",
              _.active_wave || 0,
              "/",
              _.total_waves || 0
            ] })
          ] }, _.filename);
        }) })
      ] }),
      /* @__PURE__ */ M.jsx(mh, { items: A.attention, action: Sl }),
      O === "fullscreen" && /* @__PURE__ */ M.jsxs(M.Fragment, { children: [
        A.focus && /* @__PURE__ */ M.jsx(Mh, { focus: A.focus, action: Sl }),
        /* @__PURE__ */ M.jsx(yh, { agents: A.active_agents }),
        A.focus && /* @__PURE__ */ M.jsxs("section", { className: al.readiness, children: [
          /* @__PURE__ */ M.jsx("h2", { children: "readiness" }),
          /* @__PURE__ */ M.jsx(Au, { status: A.focus.readiness.status }),
          /* @__PURE__ */ M.jsxs("p", { children: [
            "review cycle ",
            A.focus.readiness.review_cycle ?? 0
          ] }),
          /* @__PURE__ */ M.jsxs("p", { children: [
            "checks: ",
            A.focus.readiness.pr_check_status || "not reported"
          ] }),
          /* @__PURE__ */ M.jsxs("p", { children: [
            "review: ",
            A.focus.readiness.pr_review_decision || "not reported"
          ] }),
          /* @__PURE__ */ M.jsxs("p", { children: [
            "verification: ",
            A.focus.readiness.last_verify_outcome || "not reported"
          ] }),
          A.focus.readiness.has_review_feedback && Sl && /* @__PURE__ */ M.jsx("button", { onClick: () => {
            var _;
            return Sl(`approve review for ${(_ = A.focus) == null ? void 0 : _.filename}`);
          }, children: "approve review" })
        ] }),
        /* @__PURE__ */ M.jsx(hh, { events: A.events ?? [] })
      ] })
    ] })
  ] }) : /* @__PURE__ */ M.jsxs("main", { className: al.root, children: [
    /* @__PURE__ */ M.jsx("p", { children: "monitor offline" }),
    /* @__PURE__ */ M.jsx("button", { onClick: () => void q(), children: "retry" })
  ] });
}
const Nv = document.getElementById("root");
Nv && ym.createRoot(Nv).render(/* @__PURE__ */ M.jsx(im.StrictMode, { children: /* @__PURE__ */ M.jsx(Oh, {}) }));
