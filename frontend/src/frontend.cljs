(ns frontend
  (:require-macros [cljs.core.async.macros :refer [go go-loop]])
  (:require ["localforage" :as localforage]
            ["@primer/octicons-react" :as octo]
            [lambdaisland.ansi :as ansi]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<! >! chan put! close! timeout] :as a]
            [cljs.pprint]
            [reagent.dom :as reagent.dom]
            [reagent.core :as reagent]
            [bide.core :as bide]
            [garden.core :as garden]
            [clojure.string :as s]
            [reagent-mui.material.box :refer [box] :rename {box mui-box}]
            [reagent-mui.material.switch-component :refer [switch] :rename {switch mui-switch}]
            [reagent-mui.material.card :refer [card] :rename {card mui-card}]
            [reagent-mui.material.text-field :refer [text-field] :rename {text-field mui-text-field}]
            [reagent-mui.material.container :refer [container] :rename {container mui-container}]
            [reagent-mui.material.icon-button :refer [icon-button] :rename {icon-button mui-icon-button}]
            [reagent-mui.material.typography :refer [typography] :rename {typography mui-typography}]
            [reagent-mui.material.linear-progress :refer [linear-progress] :rename {linear-progress mui-linear-progress}]))

(set! *warn-on-infer* true)

(defn blur-active []
  (.blur js/document.activeElement))

(def adapt reagent/adapt-react-class)

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
    :last-distance-from-bottom 0
    :log-direction true
    :cmd-focus false
    :key-listener false
    :mouse-listener false
    :cmd-text ""
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
   [:.MuiIconButton-root:hover {:color "#586e75"}]
   [".menu-button .MuiSvgIcon-root" {:width "40px"
                                     :height "40px"}]))

(def card-style
  {:style {:padding "20px"
           :margin-bottom "10px"
           :overflow-x :scroll}
   :class "bg-color"})

(def card-style-flex (assoc-in card-style [:style :display] :flex))

(defn auth? []
  (not (s/blank? (:auth @state))))

(defn mousedown-listener [e]
  nil)

(defn navigate-to [page]
  (let [a (js/document.createElement "a")]
    (.setAttribute a "href", (str "/#" page))
    (.click a)
    (.remove a)))

(def max-retries 7)

(defn exec-api-post [cmd]
  (go-loop [i 0]
    (let [resp (<! (http/post "/api/exec"
                              {:headers {"auth" (:auth @state)}
                               :json-params {:argv ["bash" "-c" cmd]}
                               :with-credentials? false}))]
      (cond
        (= 401 (:status resp)) (do (swap! state update-in [:history] butlast)
                                   (swap! state update-in [:events] butlast)
                                   (swap! state assoc :auth "")
                                   (swap! state assoc :loading false)
                                   (throw "bad auth"))
        (= 200 (:status resp)) (:uid (:body resp))
        (< i max-retries) (do (<! (a/timeout (* i 100)))
                              (recur (inc i)))
        :else (do (swap! state assoc :loading false)
                  (throw "failed after several tries"))))))

(defn exec-api-get [uid range-start]
  (go-loop [i 0]
    (let [resp (<! (http/get "/api/exec"
                             {:query-params {:uid uid
                                             :range-start range-start}
                              :headers {"auth" (:auth @state)}
                              :with-credentials? false}))]
      (cond
        (= 200 (:status resp)) resp
        (< i max-retries) (do (<! (a/timeout (* i 100)))
                              (recur (inc i)))
        :else (throw "failed after several tries")))))

(defn s3-log-get [log-url range-start]
  (go-loop [i 0]
    (let [resp (<! (http/get log-url {:with-credentials? false
                                      :headers {"range" (str "bytes=" range-start "-")}}))]
      (cond
        (#{200 206} (:status resp)) (:body resp)
        (#{403 416} (:status resp)) nil
        (< i max-retries) (do (<! (a/timeout (* i 100)))
                              (recur (inc i)))
        :else (throw "failed after several tries")))))

(def max-events 1024)

(defn byte-size [s]
  (.-length (.encode (js/TextEncoder.) s)))

(defn scroll-to-cmd []
  (let [el (js/document.getElementById "cmd")]
    (.scrollIntoView el #js {"behavior" "smooth"
                             "block" "center"
                             "inline" "center"})))

(defn submit-cmd []
  (let [cmd (:cmd-text @state)]
    (swap! state assoc :tail true)
    (cond
      ;;
      (= "history clear" cmd)
      (do (swap! state assoc :history [])
          (swap! state assoc :cmd-text "")
          (swap! state assoc :offset 0))
      ;;
      (= "history" cmd)
      (do (swap! state update-in [:history] conj cmd)
          (swap! state update-in [:events] #(vec (take-last max-events (conj % (s/join "\n" (take-last 128 (mapv (fn [[i l]]
                                                                                                                   (str (inc i) ". " l))
                                                                                                                 (map vector (range) (:history @state)))))))))
          (swap! state assoc :cmd-text "")
          (swap! state assoc :offset 0))
      ;;
      (= "clear" cmd)
      (do (swap! state assoc :events [])
          (swap! state assoc :cmd-text "")
          (swap! state assoc :offset 0))
      ;;
      :else
      (go (swap! state update-in [:history] conj cmd)
          (swap! state update-in [:events] #(vec (take-last max-events (conj % (str ">> " cmd)))))
          (swap! state assoc :loading true)
          (swap! state assoc :cmd-focus false)
          (swap! state assoc :cmd-text "")
          (swap! state assoc :cmd-text "")
          (swap! state assoc :offset 0)
          (let [uid (<! (exec-api-post cmd))]
            (loop [range-start 0]
              (let [resp (<! (exec-api-get uid range-start))]
                (when (= 200 (:status resp))
                  (if-let [exit (:exit (:body resp))]
                    (swap! state #(-> %
                                    (update-in [:events] conj (str "exit: " exit))
                                    (assoc :loading false)))
                    (if-let [data (<! (s3-log-get (:url (:body resp)) range-start))]
                      (do (swap! state update-in [:events] #(vec (take-last max-events (conj % data))))
                          (<! (a/timeout 0))
                          (recur (+ range-start (byte-size data))))
                      (do (<! (a/timeout 3000))
                          (recur range-start))))))))))))

(defn keydown-listener [e]
  (cond
    (= "Enter" (event-key e)) (when (:cmd-focus @state)
                                (submit-cmd)
                                (prevent-default e))
    (= "ArrowUp" (event-key e)) (do (swap! state update-in [:offset] #(min (inc %) (count (:history @state))))
                                    (prevent-default e))
    (= "ArrowDown" (event-key e)) (do (swap! state update-in [:offset] #(max 0 (dec %)))
                                      (prevent-default e))
    (#{"INPUT" "TEXTAREA"} (.-tagName js/document.activeElement)) nil
    (= "/" (event-key e)) (when-not (:cmd-focus @state)
                            (swap! state merge {:cmd-focus true})
                            (prevent-default e))
    :else nil #_(log :key (event-key e))))

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

(defwatch :history
  (fn [history]
    (lf-set "history" (clj->js (take-last 1024 history)))))

(defwatch :offset
  (fn [val]
    (if (= 0 val)
      (swap! state assoc :cmd-text "")
      (let [n (dec val)]
        (swap! state assoc :cmd-text (nth (reverse (:history @state)) n))))))

(defwatch :cmd-focus
  (fn [val]
    (when val
      (focus (first (js/document.querySelectorAll "#cmd"))))))

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
        (swap! state assoc :cmd-text (js/decodeURIComponent cmd))
        (submit-cmd))
      nil)))

(defn with-random-key [xs]
  (map #(with-meta % {:key (str (js/Math.random))}) xs))

(defn component-events []
  [:<>
   (for [[i event] (map vector (range) (if (:log-direction @state)
                                         (reverse (:events @state))
                                         (:events @state)))]
     ^{:key i} [mui-card (assoc-in card-style [:style :white-space] :pre)
                [:<> (with-random-key (ansi/text->hiccup (if (:log-direction @state)
                                                           (s/join "\n" (reverse (s/split-lines event)))
                                                           event)))]])])

(defn component-cmd []
  (if (auth?)
    (let [_ (init-on-first-load)
          prompt [mui-card (-> card-style-flex
                             (merge {:id "cmd"})
                             (update-in [:style] merge {:padding 0
                                                        :padding-left "10px"
                                                        :align-items :center}))
                  [mui-switch {:size :small
                               :checked (:log-direction @state)
                               :on-change #(do (swap! state update-in [:log-direction] not)
                                               (blur-active))}]
                  [mui-typography {:style {:margin-top "3px"
                                           :margin-left "5px"
                                           :margin-right "7px"}}
                   (if (:log-direction @state)
                     [(adapt octo/ArrowUpIcon)]
                     [(adapt octo/ArrowDownIcon)])]
                  (if (:loading @state)
                    [mui-linear-progress {:style {:width "100%"
                                                  :height "21px"
                                                  :margin-left "10px"
                                                  :margin-right "10px"
                                                  :margin-top "20px"
                                                  :margin-bottom "20px"}}]
                    [mui-text-field {:label "aws-exec"
                                     :autoComplete "off"
                                     :spellCheck false
                                     :multiline true
                                     :fullWidth true
                                     :autoFocus true
                                     :focused (:cmd-focus @state)
                                     :value (:cmd-text @state)
                                     :on-focus #(swap! state assoc :cmd-focus true)
                                     :on-blur #(swap! state assoc :cmd-focus false)
                                     :on-change #(swap! state assoc :cmd-text (target-value %))
                                     :style {:background-color "rgb(240,240,240)"
                                             :margin "5px"}}])]]
      (if (:log-direction @state)
        [:<>
         prompt
         [component-events]]
        [:<>
         [component-events]
         prompt]))
    [:form
     [mui-card card-style
      [mui-text-field {:label "paste auth here"
                       :value (:auth @state)
                       :type :password
                       :on-change #(swap! state assoc :auth (target-value %))
                       :style {:width "100%"}} ]]]))

(defn component-root []
  [:<>
   [:style style]
   [mui-container {:id "content" :style {:padding 0 :margin-top "10px"}}
    [component-cmd]]])

(defn reagent-render []
  (reagent.dom/render [component-root] (js/document.getElementById "app")))

(defn ^:dev/after-load main []
  (go (document-listener "keydown" keydown-listener)
      (document-listener "mousedown" mousedown-listener)
      (<! (lf-set-backend))
      (when-let [auth (<! (lf-get "auth"))]
        (swap! state assoc :auth auth))
      (:history @state)
      (when-let [history (<! (lf-get "history"))]
        (swap! state assoc :history (js->clj history)))
      (reagent-render)))
