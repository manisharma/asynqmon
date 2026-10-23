import axios from "axios";
import {
  archiveAggregatingTask,
  archiveAllAggregatingTasks,
  batchArchiveAggregatingTasks,
  batchDeleteAggregatingTasks,
  batchRunAggregatingTasks,
  deleteAggregatingTask,
  deleteAllAggregatingTasks,
  listAggregatingTasks,
  runAggregatingTask,
  runAllAggregatingTasks,
} from "./api";

jest.mock("axios");
const mockedAxios = axios as jest.MockedFunction<typeof axios>;
const queue = "gcp:lans:1582:default:get";

beforeEach(() => {
  window.ROOT_PATH = "/monitor";
  mockedAxios.mockReset();
  mockedAxios.mockResolvedValue({
    data: {},
    status: 200,
    statusText: "OK",
    headers: {},
    config: {},
  });
});

const endpoints = [
  {
    name: "list",
    method: "get",
    suffix: "?page=2&size=10",
    invoke: (group: string) =>
      listAggregatingTasks(queue, group, { page: 2, size: 10 }),
  },
  {
    name: "delete",
    method: "delete",
    suffix: "/task-id",
    invoke: (group: string) => deleteAggregatingTask(queue, group, "task-id"),
  },
  {
    name: "batch delete",
    method: "post",
    suffix: ":batch_delete",
    invoke: (group: string) =>
      batchDeleteAggregatingTasks(queue, group, ["task-id"]),
  },
  {
    name: "delete all",
    method: "delete",
    suffix: ":delete_all",
    invoke: (group: string) => deleteAllAggregatingTasks(queue, group),
  },
  {
    name: "run",
    method: "post",
    suffix: "/task-id:run",
    invoke: (group: string) => runAggregatingTask(queue, group, "task-id"),
  },
  {
    name: "batch run",
    method: "post",
    suffix: ":batch_run",
    invoke: (group: string) => batchRunAggregatingTasks(queue, group, ["task-id"]),
  },
  {
    name: "run all",
    method: "post",
    suffix: ":run_all",
    invoke: (group: string) => runAllAggregatingTasks(queue, group),
  },
  {
    name: "archive",
    method: "post",
    suffix: "/task-id:archive",
    invoke: (group: string) => archiveAggregatingTask(queue, group, "task-id"),
  },
  {
    name: "batch archive",
    method: "post",
    suffix: ":batch_archive",
    invoke: (group: string) =>
      batchArchiveAggregatingTasks(queue, group, ["task-id"]),
  },
  {
    name: "archive all",
    method: "post",
    suffix: ":archive_all",
    invoke: (group: string) => archiveAllAggregatingTasks(queue, group),
  },
];

describe.each([
  "plain-group",
  "dataform.googleapis.com/compilation_result_action-1582-list-asia-northeast1-a",
  "literal%2F/space +?#",
])("aggregation group %s", (group) => {
  test.each(endpoints)(
    "$name encodes the group as one segment",
    async (endpoint) => {
      await endpoint.invoke(group);
      expect(mockedAxios).toHaveBeenCalledWith(
        expect.objectContaining({
          method: endpoint.method,
          url: `http://localhost:8080${window.ROOT_PATH}/api/queues/${queue}/groups/${encodeURIComponent(group)}/aggregating_tasks${endpoint.suffix}`,
        })
      );
    }
  );
});
