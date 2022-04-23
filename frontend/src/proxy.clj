(ns proxy
  (:require [clojure.string :as s]))

(defn proxy-predicate [ex config]
  (tap> [:proxy-predicate ex config])
  (s/starts-with? (.getRequestPath ex) "/api/"))
