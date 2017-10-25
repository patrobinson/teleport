/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import reactor from 'app/reactor';
import auth from 'app/services/auth';
import localStorage from 'app/services/localStorage';
import history from 'app/services/history';
import session, { BearerToken } from 'app/services/session';
import cfg from 'app/config';
import api from 'app/services/api';
import Logger from 'app/lib/logger';
import  * as AT from './../restApi/constants';
import restApiActions from './../restApi/actions';
import { RECEIVE_INVITE } from './actionTypes';  

const logger = Logger.create('flux/user/actions');

const actions = {
  
  fetchInvite(inviteToken){
    const path = cfg.api.getInviteUrl(inviteToken);
    restApiActions.start(AT.FETCHING_INVITE);    
    api.get(path).done(invite=>{
      restApiActions.success(AT.FETCHING_INVITE);
      reactor.dispatch(RECEIVE_INVITE, invite);
    })
    .fail(err => {
      let msg = api.getErrorText(err);        
      restApiActions.fail(AT.FETCHING_INVITE, msg);
    });
  },

  ensureUser(nextState, replace, cb) {        
    session.ensureSession()      
      .fail(() => {                          
        const redirectUrl = history.createRedirect(nextState.location);
        const search = `?redirect_uri=${redirectUrl}`;        
        // navigate to login
        replace({
          pathname: cfg.routes.login,
          search
        });                
      })
      .always(() => {
        cb();
      })
  },
  
  acceptInvite(name, psw, token, inviteToken){    
    const promise = auth.acceptInvite(name, psw, token, inviteToken);
    actions._handleAcceptInvitePromise(promise);
  },

  acceptInviteWithU2f(name, psw, inviteToken) {
    const promise = auth.acceptInviteWithU2f(name, psw, inviteToken);
    return actions._handleAcceptInvitePromise(promise);
  },
  
  loginWithSso(providerName, providerType) {
    let redirectUrl = history.extractRedirect();
    redirectUrl = history.ensureBaseUrl(redirectUrl);
    history.push(cfg.api.getSsoUrl(redirectUrl, providerName, providerType), true);
  },
  
  loginWithU2f(user, password) {
    const promise = auth.loginWithU2f(user, password);
    actions._handleLoginPromise(promise);
  },

  login(user, password, token) {
    const promise = auth.login(user, password, token);
    actions._handleLoginPromise(promise);              
  },

  logout() {
    session.logout();
  },

  changePasswordWithU2f(oldPsw, newPsw) {
    const promise = auth.changePasswordWithU2f(oldPsw, newPsw);
    actions._handleChangePasswordPromise(promise);    
  },

  changePassword(oldPass, newPass, token){        
    const promise = auth.changePassword(oldPass, newPass, token);
    actions._handleChangePasswordPromise(promise);    
  },

  resetPasswordChangeAttempt() {
    restApiActions.clear(AT.TRYING_TO_CHANGE_PSW);
  },

  _handleChangePasswordPromise(promise) {
    restApiActions.start(AT.TRYING_TO_CHANGE_PSW);    
    return promise
      .done(() => {                
        restApiActions.success(AT.TRYING_TO_CHANGE_PSW);            
      })
      .fail(err => {
        const msg = api.getErrorText(err);        
        logger.error('change password', err);
        restApiActions.fail(AT.TRYING_TO_CHANGE_PSW, msg);
      })        
  },

  _handleAcceptInvitePromise(promise) {
    restApiActions.start(AT.TRYING_TO_SIGN_UP);    
    return promise
      .done(() => {                
        history.push(cfg.routes.app, true);        
      })
      .fail(err => {
        const msg = api.getErrorText(err);        
        logger.error('accept invite', err);
        restApiActions.fail(AT.TRYING_TO_SIGN_UP, msg);
      })        
  },

  _handleLoginPromise(promise) {
    restApiActions.start(AT.TRYING_TO_LOGIN);
    promise
      .done(json => {        
        // needed for devServer only
        localStorage.setBearerToken(new BearerToken(json))        
        const url = history.extractRedirect();
        history.push(url, true);        
      })
      .fail(err => {
        const msg = api.getErrorText(err);
        logger.error('login', err);
        restApiActions.fail(AT.TRYING_TO_LOGIN, msg);
      })
  }
}
  
export default actions;
