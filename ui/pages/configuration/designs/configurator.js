import React from "react";
import { NoSsr, withStyles } from "@material-ui/core";
import { updatebetabadge, updatepagepath, updatepagetitle } from "../../../lib/store";
import { connect } from "react-redux";
import { bindActionCreators } from 'redux';
import Head from 'next/head';
import { getPath } from "../../../lib/path";
import DesignConfigurator from "../../../components/configuratorComponents/MeshModel";

const styles = {
  paper : {
    maxWidth : '90%',
    margin : 'auto',
    overflow : 'hidden',
  }
};

class DesignConfiguratorPage extends React.Component {
  componentDidMount() {
    console.log(`path: ${getPath()}`);
    this.props.updatepagepath({ path : getPath(), isBeta : true, title : "Configure Design" });
    this.props.updatepagetitle({ title : "Configure Design" })
    this.props.updatebetabadge({ isBeta : true })
  }

  render() {
    return (
      <NoSsr>
        <Head>
          <title>Designs Configurator</title>
        </Head>
        <DesignConfigurator />
      </NoSsr>
    );
  }
}

const mapDispatchToProps = dispatch => ({
  updatepagepath : bindActionCreators(updatepagepath, dispatch),
  updatepagetitle : bindActionCreators(updatepagetitle, dispatch),
  updatebetabadge : bindActionCreators(updatebetabadge, dispatch)
})

export default withStyles(styles)(connect(
  null,
  mapDispatchToProps
)(DesignConfiguratorPage));