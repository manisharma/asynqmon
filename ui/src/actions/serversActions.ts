import { Dispatch } from "redux";
import { listServers, ListQueuesOptions, ListServersResponse } from "../api";
import { toErrorString, toErrorStringWithHttpStatus } from "../utils";

// List of server related action types.
export const LIST_SERVERS_BEGIN = "LIST_SERVERS_BEGIN";
export const LIST_SERVERS_SUCCESS = "LIST_SERVERS_SUCCESS";
export const LIST_SERVERS_ERROR = "LIST_SERVERS_ERROR";

interface ListServersBeginAction {
  type: typeof LIST_SERVERS_BEGIN;
}
interface ListServersSuccessAction {
  type: typeof LIST_SERVERS_SUCCESS;
  payload: ListServersResponse;
}
interface ListServersErrorAction {
  type: typeof LIST_SERVERS_ERROR;
  error: string; // error description
}

// Union of all server related actions.
export type ServersActionTypes =
  | ListServersBeginAction
  | ListServersSuccessAction
  | ListServersErrorAction;

export function listServersAsync(options: ListQueuesOptions = {}) {
  return async (dispatch: Dispatch<ServersActionTypes>) => {
    dispatch({ type: LIST_SERVERS_BEGIN });
    try {
      const response = await listServers(options);
      dispatch({
        type: LIST_SERVERS_SUCCESS,
        payload: response,
      });
    } catch (error) {
      console.error(`listServersAsync: ${toErrorStringWithHttpStatus(error)}`);
      dispatch({
        type: LIST_SERVERS_ERROR,
        error: toErrorString(error),
      });
    }
  };
}
