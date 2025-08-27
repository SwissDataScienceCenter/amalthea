package controller

// Represents the state of the remote session
type RemoteSessionState string

const Running RemoteSessionState = "Running"
const Failed RemoteSessionState = "Failed"
const Completed RemoteSessionState = "Completed"
const NotReady RemoteSessionState = "NotReady"
