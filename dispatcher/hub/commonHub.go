package hub

import (
	"context"
	"sync"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/notifier-go/data"
	"github.com/ElrondNetwork/notifier-go/dispatcher"
	"github.com/ElrondNetwork/notifier-go/filters"
	"github.com/google/uuid"
)

var log = logger.GetOrCreate("hub")

type commonHub struct {
	rwMut              sync.RWMutex
	filter             filters.EventFilter
	subscriptionMapper *dispatcher.SubscriptionMapper
	dispatchers        map[uuid.UUID]dispatcher.EventDispatcher
	register           chan dispatcher.EventDispatcher
	unregister         chan dispatcher.EventDispatcher
	broadcast          chan data.BlockEvents
	broadcastRevert    chan data.RevertBlock
	broadcastFinalized chan data.FinalizedBlock
	cancelFunc         func()
}

// NewCommonHub creates a new commonHub instance
func NewCommonHub(eventFilter filters.EventFilter) *commonHub {
	return &commonHub{
		rwMut:              sync.RWMutex{},
		filter:             eventFilter,
		subscriptionMapper: dispatcher.NewSubscriptionMapper(),
		dispatchers:        make(map[uuid.UUID]dispatcher.EventDispatcher),
		register:           make(chan dispatcher.EventDispatcher),
		unregister:         make(chan dispatcher.EventDispatcher),
		broadcast:          make(chan data.BlockEvents),
		broadcastRevert:    make(chan data.RevertBlock),
		broadcastFinalized: make(chan data.FinalizedBlock),
	}
}

// Run is launched as a goroutine and listens for events on the exposed channels
func (wh *commonHub) Run() {
	var ctx context.Context
	ctx, wh.cancelFunc = context.WithCancel(context.Background())

	go wh.run(ctx)
}

func (wh *commonHub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("commonHub is stopping...")
			return

		case events := <-wh.broadcast:
			wh.handleBroadcast(events)

		case revertEvent := <-wh.broadcastRevert:
			wh.handleRevertBroadcast(revertEvent)

		case finalizedEvent := <-wh.broadcastFinalized:
			wh.handleFinalizedBroadcast(finalizedEvent)

		case dispatcherClient := <-wh.register:
			wh.registerDispatcher(dispatcherClient)

		case dispatcherClient := <-wh.unregister:
			wh.unregisterDispatcher(dispatcherClient)
		}
	}
}

// Subscribe is used by a dispatcher to send a dispatcher.SubscribeEvent
func (wh *commonHub) Subscribe(event dispatcher.SubscribeEvent) {
	wh.subscriptionMapper.MatchSubscribeEvent(event)
}

// Broadcast handles block events pushed by producers into the broadcast channel
// Upon reading the channel, the hub notifies the registered dispatchers, if any
func (wh *commonHub) Broadcast(events data.BlockEvents) {
	wh.broadcast <- events
}

// BroadcastRevert handles revert event pushed by producers into the broadcast channel
// Upon reading the channel, the hub notifies the registered dispatchers, if any
func (wh *commonHub) BroadcastRevert(event data.RevertBlock) {
	wh.broadcastRevert <- event
}

// BroadcastFinalized handles finalized event pushed by producers into the broadcast channel
// Upon reading the channel, the hub notifies the registered dispatchers, if any
func (wh *commonHub) BroadcastFinalized(event data.FinalizedBlock) {
	wh.broadcastFinalized <- event
}

// RegisterEvent will send event to a receive-only channel used to register dispatchers
func (wh *commonHub) RegisterEvent(event dispatcher.EventDispatcher) {
	wh.register <- event
}

// UnregisterEvent will send event to a receive-only channel used by a dispatcher to signal it has disconnected
func (wh *commonHub) UnregisterEvent(event dispatcher.EventDispatcher) {
	wh.unregister <- event
}

func (wh *commonHub) handleBroadcast(blockEvents data.BlockEvents) {
	subscriptions := wh.subscriptionMapper.Subscriptions()

	dispatchersMap := make(map[uuid.UUID][]data.Event)
	mapEventToDispatcher := func(id uuid.UUID, e data.Event) {
		dispatchersMap[id] = append(dispatchersMap[id], e)
	}

	for _, event := range blockEvents.Events {
		for _, subscription := range subscriptions {
			if wh.filter.MatchEvent(subscription, event) {
				mapEventToDispatcher(subscription.DispatcherID, event)
			}
		}
	}

	wh.rwMut.RLock()
	defer wh.rwMut.RUnlock()
	for id, eventValues := range dispatchersMap {
		if d, ok := wh.dispatchers[id]; ok {
			d.PushEvents(eventValues)
		}
	}
}

func (wh *commonHub) handleRevertBroadcast(revertBlock data.RevertBlock) {
}

func (wh *commonHub) handleFinalizedBroadcast(finalizedBlock data.FinalizedBlock) {
}

func (wh *commonHub) registerDispatcher(d dispatcher.EventDispatcher) {
	wh.rwMut.Lock()
	defer wh.rwMut.Unlock()

	if _, ok := wh.dispatchers[d.GetID()]; ok {
		return
	}

	wh.dispatchers[d.GetID()] = d

	log.Info("registered new dispatcher", "dispatcherID", d.GetID())
}

func (wh *commonHub) unregisterDispatcher(d dispatcher.EventDispatcher) {
	wh.rwMut.Lock()
	defer wh.rwMut.Unlock()

	if _, ok := wh.dispatchers[d.GetID()]; ok {
		delete(wh.dispatchers, d.GetID())
	}

	log.Info("unregistered dispatcher", "dispatcherID", d.GetID(), "unsubscribing", true)

	wh.subscriptionMapper.RemoveSubscriptions(d.GetID())
}

// Close will close the goroutine and channels
func (wh *commonHub) Close() error {
	if wh.cancelFunc != nil {
		wh.cancelFunc()
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wh *commonHub) IsInterfaceNil() bool {
	return wh == nil
}
