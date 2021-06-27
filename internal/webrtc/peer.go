package webrtc

import (
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

// how long is can take between sending offer and connecting
const offerTimeout = 10 * time.Second

type WebRTCPeerCtx struct {
	api         *webrtc.API
	connection  *webrtc.PeerConnection
	dataChannel *webrtc.DataChannel
	changeVideo func(videoID string) error
}

func (peer *WebRTCPeerCtx) CreateOffer(ICETrickle bool, ICERestart bool) (*webrtc.SessionDescription, error) {
	// offer timeout
	go func() {
		time.Sleep(offerTimeout)

		// already disconnected
		if peer.connection.ConnectionState() == webrtc.PeerConnectionStateClosed {
			return
		}

		// not connected
		if peer.connection.ConnectionState() != webrtc.PeerConnectionStateConnected {
			log.Warn().Msg("connection timeouted, closing")
			peer.connection.Close()
		}
	}()

	offer, err := peer.connection.CreateOffer(&webrtc.OfferOptions{
		ICERestart: ICERestart,
	})
	if err != nil {
		return nil, err
	}

	if !ICETrickle {
		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(peer.connection)

		if err := peer.connection.SetLocalDescription(offer); err != nil {
			return nil, err
		}

		<-gatherComplete
	} else {
		if err := peer.connection.SetLocalDescription(offer); err != nil {
			return nil, err
		}
	}

	return peer.connection.LocalDescription(), nil
}

func (peer *WebRTCPeerCtx) SignalAnswer(sdp string) error {
	return peer.connection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  sdp,
		Type: webrtc.SDPTypeAnswer,
	})
}

func (peer *WebRTCPeerCtx) SignalCandidate(candidate webrtc.ICECandidateInit) error {
	return peer.connection.AddICECandidate(candidate)
}

func (peer *WebRTCPeerCtx) SetVideoID(videoID string) error {
	return peer.changeVideo(videoID)
}

func (peer *WebRTCPeerCtx) Destroy() error {
	if peer.connection == nil || peer.connection.ConnectionState() != webrtc.PeerConnectionStateConnected {
		return nil
	}

	return peer.connection.Close()
}
