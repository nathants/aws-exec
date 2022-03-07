(ns frontend
  (:require-macros [cljs.core.async.macros :refer [go go-loop]])
  (:require ["libsodium-wrappers" :as sodium]
            ["localforage" :as localforage]
            [lambdaisland.ansi :as ansi]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<! >! chan put! close! timeout] :as a]
            [cljs.pprint]
            [reagent.dom :as reagent.dom]
            [reagent.core :as reagent]
            [bide.core :as bide]
            [garden.core :as garden]
            [clojure.string :as s]
            [reagent-mui.material.card :refer [card]]
            [reagent-mui.material.text-field :refer [text-field]]
            [reagent-mui.material.container :refer [container]]
            [reagent-mui.material.linear-progress :refer [linear-progress]]))

(set! *warn-on-infer* true)

(defn log [& args]
  (apply js/console.log (map clj->js args)))

(defn event-key ^str [^js/KeyboardEvent e]
  (if e
    (.-key e)
    ""))

(defn focus [^js/Object e]
  (.focus e))

(defn target-value [^js/Event e]
  (.-value (.-target e)))

(defn prevent-default [^js/Event e]
  (.preventDefault e))

(defn to-uint8-array [string]
  (.encode (js/TextEncoder.) string))

(defn to-hex [uint8-array]
  (.to_hex sodium uint8-array))

(def sodium-ready
  (let [c (chan)]
    (.then (.-ready sodium) #(close! c))
    c))

(defn blake2b-32 [string]
  (to-hex (.crypto_generichash sodium 32 (to-uint8-array string))))

(defn lf-set-backend []
  (let [c (chan)]
    (-> (.setDriver localforage (.-INDEXEDDB localforage))
      (.then #(close! c))
      (.catch #(do (log :error %)
                   (put! c false))))
    c))

(defn lf-clear []
  (let [c (chan)]
    (-> (.clear localforage)
      (.then #(close! c))
      (.catch #(do (log :error %)
                   (put! c false))))
    c))

(defn lf-set [k v]
  (let [c (chan)]
    (-> (.setItem localforage k v)
      (.then #(close! c))
      (.catch #(do (log :error %)
                   (put! c false))))
    c))

(defn lf-get [k]
  (let [c (chan)]
    (-> (.getItem localforage k)
      (.then #(put! c (or % false)))
      (.catch #(do (log :error %)
                   (put! c false))))
    c))

(defn lf-rm [k]
  (let [c (chan)]
    (-> (.removeItem localforage k)
      (.then #(put! c true))
      (.catch #(do (log :error %)
                   (put! c false))))
    c))

(defonce state
  (reagent/atom
   {:username ""
    :password ""
    :history []
    :offset 0
    :search-focus false
    :key-listener false
    :mouse-listener false
    :search-text ""
    :events []
    :loading false
    :page nil
    :first-load? true}))

(def style
  (garden/css
   [:body {:background-color "rgb(240, 240, 240)"}]
   [:.bg-color {:background-color "rgb(230, 230, 230)"}]
   ["*" {:font-family "monospace !important"}]
   [:.MuiIconButton-root {:border-radius "10%"}]
   [:.MuiAppBar-colorPrimary {:background-color "rgb(230, 230, 230)"}]
   [".menu-button .MuiSvgIcon-root" {:width "40px"
                                     :height "40px"}]))

(def card-style
  {:style {:padding "20px"
           :margin-bottom "10px"
           :overflow-x :scroll}
   :class "bg-color"})

(defn auth? []
  (not (s/blank? (:auth @state))))

(defn scroll-down []
  (mapv #(go
           (<! (a/timeout (* % 5)))
           (js/window.scrollTo 0 js/document.body.scrollHeight))
        (range 20)))

(defn mousedown-listener [e]
  nil)

(defn navigate-to [page]
  (let [a (js/document.createElement "a")]
    (.setAttribute a "href", (str "/#" page))
    (.click a)
    (.remove a)))

(def api-url "") ;; or "https://$PROJECT_DOMAIN" when using bin/dev.sh

(defn exec-api-post [cmd]
  (go-loop [i 0]
    (let [resp (<! (http/post (str api-url "/api/exec")
                              {:headers {"auth" (blake2b-32 (:auth @state))}
                               :json-params {:argv ["bash" "-c" cmd]}
                               :with-credentials? false}))]
      (cond
        (= 401 (:status resp)) (do (swap! state update-in [:history] butlast)
                                   (swap! state update-in [:events] butlast)
                                   (swap! state assoc :auth "")
                                   (swap! state assoc :loading false)
                                   (throw "bad auth"))
        (= 200 (:status resp)) (:uid (:body resp))
        (< i 7) (do (<! (a/timeout (* i 100)))
                    (recur (inc i)))
        :else (do (swap! state assoc :loading false)
                  (throw "failed after several tries"))))))

(defn exec-api-get [uid increment]
  (go-loop [i 0]
    (let [resp (<! (http/get (str api-url "/api/exec")
                             {:query-params {:uid uid
                                             :increment increment}
                              :headers {"auth" (blake2b-32 (:auth @state))}
                              :with-credentials? false}))]
      (when (= 7 i)
        (swap! state assoc :loading false)
        (throw "failed after several tries"))
      (cond
        (= 200 (:status resp)) resp
        (= 409 (:status resp)) resp
        :else (do (<! (a/timeout (* i 100)))
                  (recur (inc i)))))))

(defn s3-log-get [log-url]
  (go-loop [i 0]
    (let [resp (<! (http/get log-url {:with-credentials? false}))]
      (when (= 7 i)
        (swap! state assoc :loading false)
        (throw "failed after several tries"))
      (cond
        (= 200 (:status resp)) resp
        :else (do (<! (a/timeout (* i 100)))
                  (recur (inc i)))))))

(def max-events 256)

(defn submit-command []
  (go
    (swap! state merge {:loading true
                        :search-focus false})
    (let [cmd (:search-text @state)
          _ (swap! state update-in [:history] conj cmd)
          _ (swap! state update-in [:events] #(vec (take-last max-events (conj % (str ">> " cmd)))))
          _ (swap! state assoc :search-text "")
          _ (swap! state assoc :offset 0)
          uid (<! (exec-api-post cmd))]
      (loop [increment 0]
        (let [resp (<! (exec-api-get uid increment))]
          (condp = (:status resp)
            200 (if-let [exit-code (:exit_code (:body resp))]
                  (swap! state #(-> %
                                  (update-in [:events] conj (str "exit code: " exit-code))
                                  (assoc :loading false)))
                  (let [new-increment (:increment (:body resp))
                        log-url (:log (:body resp))
                        event (:body (<! (s3-log-get log-url)))]
                    (swap! state update-in [:events] #(vec (take-last max-events (conj % event))))
                    (recur new-increment)))
            409 (do (<! (a/timeout 1000))
                    (recur increment))))))))

(defn keydown-listener [e]
  (cond
    (= "Enter" (event-key e)) (when (:search-focus @state)
                                (submit-command)
                                (prevent-default e))
    (#{"INPUT" "TEXTAREA"} (.-tagName js/document.activeElement)) nil
    (= "/" (event-key e)) (when-not (:search-focus @state)
                            (swap! state merge {:search-focus true})
                            (prevent-default e))
    (= "ArrowUp" (event-key e)) (do (swap! state update-in [:offset] inc)
                                    (prevent-default e))
    (= "ArrowDown" (event-key e)) (do (swap! state update-in [:offset] dec)
                                      (prevent-default e))
    :else (log :key (event-key e))))

(defn url []
  (last (s/split js/window.location.href #"#/")))

(defn href-parts []
  (s/split (url) #"/"))

(defn defwatch [key f]
  (add-watch state key (fn [key atom old new]
                         (when (not= (get old key)
                                     (get new key))
                           (f  (get new key))))))

(defwatch :auth
  (fn [text]
    (lf-set "auth" text)))

(defwatch :loading
  (fn [_]
    (scroll-down)))

(defwatch :events
  (fn [_]
    (scroll-down)))

(defwatch :offset
  (fn [val]
    (when (not= 0 val)
      (let [n (mod (dec val) (count (:history @state)))]
        (swap! state assoc :search-text (nth (:history @state) n))))))

(defwatch :search-focus
  (fn [val]
    (when val
      (focus (first (js/document.querySelectorAll "#search"))))))

(defn on-navigate [component data]
  (swap! state merge {:page component :parts (href-parts)}))

(defn document-listener [name f]
  (let [key (keyword (str name "-listener"))]
    (when-not (key @state)
      (.addEventListener js/document name f)
      (swap! state assoc key true))))

(defn init-on-first-load []
  (when (:first-load? @state)
    (swap! state assoc :first-load? false)
    (let [params (last (s/split js/window.location.href #"\?"))
          params (s/split params #"&")
          params (map #(s/split % #"=") params)
          params (filter #(= 2 (count %)) params)
          params (flatten params)
          params (apply hash-map params)]
      (when-let [cmd (get params "cmd")]
        (log "hello" cmd)
        (swap! state assoc :search-text (js/decodeURIComponent cmd))
        (submit-command))
      nil)))

(defn with-random-key [xs]
  (map #(with-meta % {:key (str (js/Math.random))}) xs))

(defn component-home []
  (if (auth?)
    [:<>
     (init-on-first-load)
     (for [[i event] (map vector (range) (:events @state))]
       ^{:key i} [card (assoc-in card-style [:style :white-space] :pre)
                  [:<> (with-random-key (ansi/text->hiccup event))]])
     (if (:loading @state)
       [card card-style
        [linear-progress {:style {:height "13px"}}] ]
       [text-field {:label "slim.ai rce"
                    :id "search"
                    :autoComplete "off"
                    :spellCheck false
                    :multiline true
                    :fullWidth true
                    :autoFocus true
                    :focused (:search-focus @state)
                    :value (:search-text @state)
                    :on-focus #(swap! state assoc :search-focus true)
                    :on-blur #(swap! state assoc :search-focus false)
                    :on-change #(swap! state assoc :search-text (target-value %))
                    :style {:width "98%"
                            :margin "1%"}}])]
    [:form
     [card card-style
      [text-field {:label "paste auth token here"
                   :value (:auth @state)
                   :type :password
                   :on-change #(swap! state assoc :auth (target-value %))
                   :style {:width "100%"}} ]]]))

(defn component-not-found []
  [:div
   [:p "404"]])

(defn component-main []
  [container {:id "content" :style {:padding 0 :margin-top "10px"}}
   [(:page @state)]])

(defn component-root []
  [:<>
   [:style style]
   [component-main]])

(def router
  [["/" component-home]
   ["(.*)" component-not-found]])

(defn start-router []
  (bide/start! (bide/router router) {:default "/"
                                     :on-navigate on-navigate
                                     :html5? false}))

(defn reagent-render []
  (reagent.dom/render [component-root] (js/document.getElementById "app")))

(defn ^:dev/after-load main []
  (go (start-router)
      (document-listener "keydown" keydown-listener)
      (document-listener "mousedown" mousedown-listener)
      (<! (lf-set-backend))
      (when-let [auth (<! (lf-get "auth"))]
        (swap! state assoc :auth auth))
      (<! sodium-ready)
      (reagent-render)))
