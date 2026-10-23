import { configureStore } from "@reduxjs/toolkit";
import { act, cleanup, render } from "@testing-library/react";
import React from "react";
import { Provider } from "react-redux";
import {
  pollIntervalChange,
  taskRowsPerPageChange,
} from "../actions/settingsActions";
import { listAggregatingTasks, ListAggregatingTasksResponse } from "../api";
import groupsReducer from "../reducers/groupsReducer";
import settingsReducer from "../reducers/settingsReducer";
import tasksReducer from "../reducers/tasksReducer";
import AggregatingTasksTable from "./AggregatingTasksTable";

jest.mock("../api", () => ({
  ...jest.requireActual("../api"),
  listAggregatingTasks: jest.fn(),
}));
jest.mock("./SyntaxHighlighter", () => () => null);

const listTasks = listAggregatingTasks as jest.MockedFunction<
  typeof listAggregatingTasks
>;
const queue = "azure:lans:1603:default:get";
const group = "Microsoft.OperationalInsights/workspaces/tables-1603-list";
let finishRequest: (response: ListAggregatingTasksResponse) => void;

beforeEach(() => {
  jest.useFakeTimers();
  listTasks.mockReset();
  listTasks.mockImplementation(
    () =>
      new Promise((resolve) => {
        finishRequest = resolve;
      })
  );
});

afterEach(() => {
  cleanup();
  jest.useRealTimers();
});

function setup() {
  const store = configureStore({
    reducer: {
      settings: settingsReducer,
      tasks: tasksReducer,
      groups: groupsReducer,
    },
  });
  store.dispatch(pollIntervalChange(8));
  store.dispatch(taskRowsPerPageChange(20));
  const table = (selectedGroup: string) => (
    <Provider store={store}>
      <AggregatingTasksTable
        queue={queue}
        selectedGroup={selectedGroup}
        totalTaskCount={100}
      />
    </Provider>
  );
  const view = render(table(group));
  return { store, table, ...view };
}

async function completeRequest() {
  await act(async () => {
    finishRequest({
      tasks: [],
      groups: [{ group, size: 100 }],
      stats: {
        queue,
        paused: false,
        size: 100,
        groups: 1,
        latency_msec: 0,
        display_latency: "0s",
        memory_usage_bytes: 0,
        active: 0,
        pending: 0,
        aggregating: 100,
        scheduled: 0,
        retry: 0,
        archived: 0,
        completed: 0,
        processed: 0,
        failed: 0,
        timestamp: "2026-10-23T00:00:00Z",
      },
    });
  });
}

function advanceTime(milliseconds: number) {
  act(() => {
    jest.advanceTimersByTime(milliseconds);
  });
}

test("request state updates do not bypass the polling interval", async () => {
  const { unmount } = setup();
  expect(listTasks).toHaveBeenCalledTimes(1);
  expect(listTasks).toHaveBeenLastCalledWith(queue, group, { page: 1, size: 20 });

  await completeRequest();
  expect(listTasks).toHaveBeenCalledTimes(1);
  advanceTime(7999);
  expect(listTasks).toHaveBeenCalledTimes(1);
  advanceTime(1);
  expect(listTasks).toHaveBeenCalledTimes(2);

  await completeRequest();
  expect(listTasks).toHaveBeenCalledTimes(2);
  advanceTime(8000);
  expect(listTasks).toHaveBeenCalledTimes(3);

  unmount();
  advanceTime(16000);
  expect(listTasks).toHaveBeenCalledTimes(3);
});

test("switching groups and page size still fetches immediately", async () => {
  const { store, table, rerender } = setup();
  await completeRequest();
  const nextGroup = "Microsoft.Compute/virtualMachines-1603-list";
  rerender(table(nextGroup));
  expect(listTasks).toHaveBeenCalledTimes(2);
  expect(listTasks).toHaveBeenLastCalledWith(queue, nextGroup, { page: 1, size: 20 });

  await completeRequest();
  act(() => {
    store.dispatch(taskRowsPerPageChange(50));
  });
  expect(listTasks).toHaveBeenCalledTimes(3);
  expect(listTasks).toHaveBeenLastCalledWith(queue, nextGroup, { page: 1, size: 50 });
});

test("changing the polling setting replaces the previous timer", async () => {
  const { store } = setup();
  await completeRequest();
  act(() => {
    store.dispatch(pollIntervalChange(15));
  });
  expect(listTasks).toHaveBeenCalledTimes(2);
  await completeRequest();
  advanceTime(14999);
  expect(listTasks).toHaveBeenCalledTimes(2);
  advanceTime(1);
  expect(listTasks).toHaveBeenCalledTimes(3);
});
