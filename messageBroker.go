package main

import (messager "popcorn/proto")

func oneIssueLogInstance(msgIndex, clientID, timestamp int64, msg string, txHash, sig []byte) *messager.LeaderIssueCmd{

	return &messager.LeaderIssueCmd{
		MsgType:   int64(messager.MessageType_LeaderIssueLogType),
		MsgIndex:  msgIndex,
		ClientID:  clientID,
		Timestamp: timestamp,
		Tx:        msg,
		TxHash:    txHash,
		Sig:       sig,
		ThreshSig: nil,
	}
}

func prepareIssueLogMessage(m *messager.LeaderIssueCmd) *messager.MessageBank{
	return &messager.MessageBank{
		CmuType:            messager.CommunicationType_Type_Internal_Issue_Cmd,
		LeaderIssueCmd:     m,
	}
}

func prepareLeaderIssueOrderMessage(txHash, infoHash, thresholdSig []byte, msgIndex, clientId, timestamp int64) *messager.MessageBank{
	
	order := &messager.LeaderIssueCmd{
		MsgType:   int64(messager.MessageType_LeaderIssueOrderType),
		MsgIndex:  msgIndex,
		ClientID:  clientId,
		Timestamp: timestamp,
		Tx:        "",
		TxHash:    txHash,
		Sig:       nil,
		ThreshSig: thresholdSig,
	}

	return &messager.MessageBank{
		CmuType			: messager.CommunicationType_Type_Internal_Issue_Cmd,
		LeaderIssueCmd	: order,
	}
}

func prepareClientConfirmation(timestamp, msgIndex int64, txHash []byte) *messager.MessageBank{
	clientConfirmation := &messager.ClientConfirmation{
		Timestamp	: timestamp,
		CommitIndex	: msgIndex,
		TxHash		: txHash,
	}

	return &messager.MessageBank{
		CmuType				: messager.CommunicationType_Type_Cluster_To_Client,
		ClientConfirmation	: clientConfirmation,
	}
}