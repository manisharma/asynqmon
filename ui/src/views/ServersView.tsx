import React, { useCallback, useState } from "react";
import { connect, ConnectedProps } from "react-redux";
import Container from "@material-ui/core/Container";
import { makeStyles } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import Typography from "@material-ui/core/Typography";
import Alert from "@material-ui/lab/Alert";
import AlertTitle from "@material-ui/lab/AlertTitle";
import TablePagination from "@material-ui/core/TablePagination";
import TextField from "@material-ui/core/TextField";
import ServersTable from "../components/ServersTable";
import { listServersAsync } from "../actions/serversActions";
import { AppState } from "../store";
import { usePolling } from "../hooks";

const useStyles = makeStyles((theme) => ({
  container: {
    paddingTop: theme.spacing(4),
    paddingBottom: theme.spacing(4),
  },
  paper: {
    padding: theme.spacing(2),
    display: "flex",
    overflow: "auto",
    flexDirection: "column",
  },
  heading: {
    paddingLeft: theme.spacing(2),
    marginBottom: theme.spacing(1),
  },
}));

function mapStateToProps(state: AppState) {
  return {
    loading: state.servers.loading,
    error: state.servers.error,
    servers: state.servers.data,
    pageSize: state.servers.size,
    total: state.servers.total,
    pollInterval: state.settings.pollInterval,
  };
}

const connector = connect(mapStateToProps, { listServersAsync });

type Props = ConnectedProps<typeof connector>;

function ServersView(props: Props) {
  const { pollInterval, listServersAsync, pageSize, total } = props;
  const classes = useStyles();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");

  const listServers = useCallback(
    () => listServersAsync({ page, size: pageSize, search }),
    [listServersAsync, page, pageSize, search]
  );
  usePolling(listServers, pollInterval);

  return (
    <Container maxWidth="lg" className={classes.container}>
      <Grid container spacing={3}>
        {props.error === "" ? (
          <Grid item xs={12}>
            <Paper className={classes.paper} variant="outlined">
              <Typography variant="h6" className={classes.heading}>
                Servers
              </Typography>
              <TextField
                label="Search hosts"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                }}
                variant="outlined"
                margin="dense"
              />
              <ServersTable servers={props.servers} />
              <TablePagination
                component="div"
                count={total}
                page={page - 1}
                onPageChange={(_, nextPage) => setPage(nextPage + 1)}
                rowsPerPage={pageSize}
                rowsPerPageOptions={[pageSize]}
              />
            </Paper>
          </Grid>
        ) : (
          <Grid item xs={12}>
            <Alert severity="error">
              <AlertTitle>Error</AlertTitle>
              Could not retrieve servers live data —{" "}
              <strong>See the logs for details</strong>
            </Alert>
          </Grid>
        )}
      </Grid>
    </Container>
  );
}

export default connector(ServersView);
